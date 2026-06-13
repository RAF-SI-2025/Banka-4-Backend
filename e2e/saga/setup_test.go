//go:build saga_e2e

// Package saga_e2e implements SG-09, SG-10 and SG-11 from the SAGA test
// specification: infrastructure failures that cannot be simulated in-process
// (paused services, network latency and partitions via Toxiproxy, and a
// SIGKILLed coordinator).
//
// The suite runs against the docker-compose dev stack with the saga overlay:
//
//	docker compose -f docker-compose-dev.yml -f docker-compose-saga-test.yml up -d --build
//	make test-saga-e2e
//
// The overlay routes trading->banking gRPC through Toxiproxy and enables the
// X-Saga-* fault-injection hook. Tests seed their own unique fixtures
// directly in the shared Postgres and talk to the trading service's public
// REST API with a minted client JWT. If the stack is not reachable the suite
// skips.
package saga_e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonjwt "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
)

var (
	repoRoot      string
	db            *gorm.DB
	tradingURL    string
	toxiproxyURL  string
	jwtSecret     string
	stackErr      error
	httpc         = &http.Client{Timeout: 90 * time.Second}
	uniqueCounter atomic.Uint64
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestMain(m *testing.M) {
	var err error
	repoRoot, err = filepath.Abs("../..")
	if err != nil {
		fmt.Println("resolve repo root:", err)
		os.Exit(1)
	}

	_ = godotenv.Load(filepath.Join(repoRoot, ".env"))

	tradingURL = envOr("SAGA_E2E_TRADING_URL", "http://localhost:"+envOr("TRADING_SERVICE_PORT", "8082"))
	toxiproxyURL = envOr("SAGA_E2E_TOXIPROXY_URL", "http://localhost:8474")
	jwtSecret = envOr("JWT_SECRET", "replace-me")

	stackErr = probeStack()
	if stackErr == nil {
		dsn := fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=disable",
			envOr("DB_PORT", "5432"), envOr("DB_USER", "postgres"), envOr("DB_PASS", "1234"), envOr("DB_NAME", "banka4"))
		db, err = gorm.Open(gormpostgres.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
		if err != nil {
			stackErr = fmt.Errorf("connect to shared postgres: %w", err)
		}
	}

	os.Exit(m.Run())
}

func probeStack() error {
	c := &http.Client{Timeout: 3 * time.Second}

	resp, err := c.Get(tradingURL + "/api/health")
	if err != nil {
		return fmt.Errorf("trading service unreachable at %s (start the stack with: docker compose -f docker-compose-dev.yml -f docker-compose-saga-test.yml up -d --build): %w", tradingURL, err)
	}
	resp.Body.Close()

	resp, err = c.Get(toxiproxyURL + "/proxies/banking_grpc")
	if err != nil {
		return fmt.Errorf("toxiproxy unreachable at %s (is the saga overlay applied?): %w", toxiproxyURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("toxiproxy proxy banking_grpc missing (status %d)", resp.StatusCode)
	}

	return nil
}

func requireStack(t *testing.T) {
	t.Helper()
	if stackErr != nil {
		t.Skipf("saga e2e stack not available: %v", stackErr)
	}
}

// --- fixture seeding ---

type fixture struct {
	contractID uint
	assetID    uint
	buyerID    uint
	sellerID   uint
	buyerAcc   string
	sellerAcc  string
	token      string
}

// seedFixture creates one exercisable OTC contract (qty=10, strike=300 RSD,
// settlement tomorrow) with unique parties, asset and bank accounts so
// parallel runs and leftover data never collide.
func seedFixture(t *testing.T, buyerBalance float64) *fixture {
	t.Helper()

	n := uint(time.Now().UnixNano()%1_000_000_000)*1000 + uint(uniqueCounter.Add(1))
	f := &fixture{
		buyerID:   n,
		sellerID:  n + 1,
		buyerAcc:  fmt.Sprintf("444%015d", n),
		sellerAcc: fmt.Sprintf("444%015d", n+1),
	}

	now := time.Now()

	// Trading side: asset, stock, seller holding with the contract reservation.
	asset := &tradeAsset{Ticker: fmt.Sprintf("SG%d", n%1_000_000_000_000), Name: "Saga E2E", AssetType: "stock"}
	require(t, db.Create(asset).Error, "seed asset")
	f.assetID = asset.AssetID

	stock := &tradeStock{AssetID: asset.AssetID, OutstandingShares: 1_000_000, DividendYield: 1}
	require(t, db.Create(stock).Error, "seed stock")

	ownership := &tradeAssetOwnership{
		UserId:         f.sellerID,
		OwnerType:      ownerTypeClient,
		AssetID:        asset.AssetID,
		Amount:         10,
		PublicAmount:   10,
		ReservedAmount: 10,
		UpdatedAt:      now,
	}
	require(t, db.Create(ownership).Error, "seed seller ownership")

	contract := &tradeOtcOptionContract{
		OtcOfferID:          n,
		BuyerID:             f.buyerID,
		SellerID:            f.sellerID,
		StockAssetID:        asset.AssetID,
		Amount:              10,
		StrikePriceRSD:      300,
		PremiumRSD:          5,
		SettlementDate:      now.Add(24 * time.Hour),
		BuyerAccountNumber:  f.buyerAcc,
		SellerAccountNumber: f.sellerAcc,
		Status:              contractStatusActive,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	require(t, db.Create(contract).Error, "seed contract")
	f.contractID = contract.OtcOptionContractID

	reservation := &tradeOtcShareReservation{
		ContractID:     contract.OtcOptionContractID,
		SellerID:       f.sellerID,
		OwnerType:      ownerTypeClient,
		StockAssetID:   asset.AssetID,
		ReservedAmount: 10,
		Status:         shareReservationActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require(t, db.Create(reservation).Error, "seed share reservation")

	// Banking side: RSD accounts for both parties.
	var currency bankCurrency
	require(t, db.Where(bankCurrency{Code: "RSD"}).
		Attrs(bankCurrency{Name: "Serbian dinar", Symbol: "RSD", Status: "Active"}).
		FirstOrCreate(&currency).Error, "ensure RSD currency")

	for _, acc := range []struct {
		number   string
		clientID uint
		balance  float64
	}{
		{f.buyerAcc, f.buyerID, buyerBalance},
		{f.sellerAcc, f.sellerID, 0},
	} {
		account := &bankAccount{
			AccountNumber:    acc.number,
			Name:             "Saga E2E",
			ClientID:         acc.clientID,
			EmployeeID:       1,
			CurrencyID:       currency.CurrencyID,
			Balance:          acc.balance,
			AvailableBalance: acc.balance,
			ExpiresAt:        now.Add(365 * 24 * time.Hour),
			Status:           "Active",
			AccountType:      "Personal",
			AccountKind:      "Current",
			Subtype:          "Standard",
		}
		require(t, db.Create(account).Error, "seed bank account "+acc.number)
	}

	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   f.buyerID,
		IdentityType: string(commonauth.IdentityClient),
		ClientID:     &f.buyerID,
	}, jwtSecret, 60)
	require(t, err, "mint buyer JWT")
	f.token = "Bearer " + token

	return f
}

func require(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// --- HTTP actions ---

// exercise fires the spec Action: POST /otc/contracts/{id}/exercise with
// optional adversarial headers. It returns the status code and decoded body.
func exercise(t *testing.T, f *fixture, headers map[string]string) (int, map[string]any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/otc/contracts/%d/exercise", tradingURL, f.contractID), bytes.NewReader(nil))
	require(t, err, "build exercise request")
	req.Header.Set("Authorization", f.token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpc.Do(req)
	require(t, err, "perform exercise request")
	defer resp.Body.Close()

	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// --- state access (spec Wait/Assert read the saga log and balances) ---

func findSaga(t *testing.T, contractID uint) *tradeOtcExecutionSaga {
	t.Helper()

	var saga tradeOtcExecutionSaga
	err := db.Where("contract_id = ?", contractID).First(&saga).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	require(t, err, "load saga")
	return &saga
}

func sagaLogEntries(t *testing.T, sagaID uint) []tradeOtcExecutionSagaLogEntry {
	t.Helper()

	var entries []tradeOtcExecutionSagaLogEntry
	require(t, db.Where("otc_execution_saga_id = ?", sagaID).
		Order("otc_execution_saga_log_entry_id ASC").Find(&entries).Error, "load saga log")
	return entries
}

func accountAvailable(t *testing.T, number string) float64 {
	t.Helper()

	var account bankAccount
	require(t, db.Where("account_number = ?", number).First(&account).Error, "load account "+number)
	return account.AvailableBalance
}

func fundsReservationStatus(t *testing.T, executionKey string) string {
	t.Helper()

	var reservation bankOtcFundsReservation
	err := db.Where("execution_id = ?", executionKey).First(&reservation).Error
	if err == gorm.ErrRecordNotFound {
		return ""
	}
	require(t, err, "load funds reservation")
	return reservation.Status
}

func isTerminal(s string) bool {
	return s == sagaStatusCompleted || s == sagaStatusFailed
}

// waitTerminal polls the saga row until it is terminal (spec Wait phase). The
// deployed worker resumes pending sagas every 15s; pump=true additionally
// re-POSTs the exercise endpoint to speed recovery up the way a retrying
// client would.
func waitTerminal(t *testing.T, f *fixture, timeout time.Duration, pump bool) *tradeOtcExecutionSaga {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		saga := findSaga(t, f.contractID)
		if saga != nil && isTerminal(saga.Status) {
			return saga
		}
		if pump && saga != nil {
			exercise(t, f, nil)
		}
		time.Sleep(1 * time.Second)
	}

	saga := findSaga(t, f.contractID)
	t.Fatalf("saga did not reach a terminal status within %s (I5 violated); last state: %+v", timeout, saga)
	return nil
}

// assertInvariants checks money conservation (I1), banking reservation
// cleanup (I3) and contract consumption (I6) for a terminal saga.
func assertInvariants(t *testing.T, f *fixture, saga *tradeOtcExecutionSaga, buyerStart float64) {
	t.Helper()

	buyer := accountAvailable(t, f.buyerAcc)
	seller := accountAvailable(t, f.sellerAcc)
	if buyer+seller != buyerStart {
		t.Errorf("I1: money not conserved: buyer=%v seller=%v start=%v", buyer, seller, buyerStart)
	}

	if rs := fundsReservationStatus(t, saga.ExecutionKey); rs == fundsReservationReserved {
		t.Errorf("I3: banking funds reservation %s still RESERVED after terminal saga", saga.ExecutionKey)
	}

	var contract tradeOtcOptionContract
	require(t, db.First(&contract, f.contractID).Error, "load contract")
	exercised := contract.Status == contractStatusExercised
	if completed := saga.Status == sagaStatusCompleted; exercised != completed {
		t.Errorf("I6: contract status %s does not match saga status %s", contract.Status, saga.Status)
	}
}

// --- chaos controls ---

func composeCmd(args ...string) *exec.Cmd {
	full := append([]string{
		"compose",
		"-f", filepath.Join(repoRoot, "docker-compose-dev.yml"),
		"-f", filepath.Join(repoRoot, "docker-compose-saga-test.yml"),
		"--project-directory", repoRoot,
	}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Env = os.Environ()
	// Compose refuses to parse the files when this port variable is unset.
	if os.Getenv("EMAIL_SERVICE_PORT") == "" {
		cmd.Env = append(cmd.Env, "EMAIL_SERVICE_PORT=8084")
	}
	return cmd
}

func compose(t *testing.T, args ...string) {
	t.Helper()

	out, err := composeCmd(args...).CombinedOutput()
	require(t, err, fmt.Sprintf("docker compose %v: %s", args, out))
}

func pauseBanking(t *testing.T) {
	t.Helper()
	compose(t, "pause", "banking_service")
	t.Cleanup(func() { _ = composeCmd("unpause", "banking_service").Run() })
}

func unpauseBanking(t *testing.T) {
	t.Helper()
	compose(t, "unpause", "banking_service")
}

func toxiproxyDo(t *testing.T, method, path string, body any) {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require(t, err, "marshal toxiproxy payload")
	}

	req, err := http.NewRequest(method, toxiproxyURL+path, bytes.NewReader(payload))
	require(t, err, "build toxiproxy request")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require(t, err, "toxiproxy request "+path)
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && !(method == http.MethodDelete && resp.StatusCode == http.StatusNotFound) {
		t.Fatalf("toxiproxy %s %s: status %d", method, path, resp.StatusCode)
	}
}

// addBankingLatency injects latency (ms) on the trading->banking link, well
// above the orchestrator's RPC timeout.
func addBankingLatency(t *testing.T, latencyMs int) {
	t.Helper()
	toxiproxyDo(t, http.MethodPost, "/proxies/banking_grpc/toxics", map[string]any{
		"name":       "saga_latency",
		"type":       "latency",
		"stream":     "downstream",
		"toxicity":   1.0,
		"attributes": map[string]any{"latency": latencyMs},
	})
	t.Cleanup(func() { removeBankingLatency(t) })
}

func removeBankingLatency(t *testing.T) {
	t.Helper()
	toxiproxyDo(t, http.MethodDelete, "/proxies/banking_grpc/toxics/saga_latency", nil)
}

// setBankingLinkEnabled simulates a network partition by disabling the proxy.
func setBankingLinkEnabled(t *testing.T, enabled bool) {
	t.Helper()
	toxiproxyDo(t, http.MethodPost, "/proxies/banking_grpc", map[string]any{"enabled": enabled})
	if !enabled {
		t.Cleanup(func() { toxiproxyDo(t, http.MethodPost, "/proxies/banking_grpc", map[string]any{"enabled": true}) })
	}
}

// waitTradingHealthy blocks until the trading service answers its health
// endpoint again (used after SIGKILL + restart).
func waitTradingHealthy(t *testing.T, timeout time.Duration) {
	t.Helper()

	c := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := c.Get(tradingURL + "/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("trading service did not become healthy within %s", timeout)
}
