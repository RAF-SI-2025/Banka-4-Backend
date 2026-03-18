package handler

import (
	"common/pkg/auth"
	"fmt"
	"net/http"

	"banking-service/internal/dto"
	"banking-service/internal/service"
	"common/pkg/errors" // Vaš custom paket za greške

	"github.com/gin-gonic/gin"
)

type LoanHandler struct {
	loanService *service.LoanService
}

func NewLoanHandler(loanService *service.LoanService) *LoanHandler {
	return &LoanHandler{loanService: loanService}
}

// SubmitLoanRequest godoc
// @Summary      Podnošenje zahteva za kredit
// @Description  Klijent podnosi zahtev za kredit. Vrši se validacija perioda otplate i valute, i računa se mesečna rata na osnovu marže banke.
// @Tags         loans
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateLoanRequest true "Podaci za zahtev kredita"
// @Success      201  {object}  dto.CreateLoanResponse
// @Failure      400  {object}  errors.AppError "Nevalidni podaci, valuta se ne poklapa ili los period otplate"
// @Failure      401  {object}  errors.AppError "Korisnik nije ulogovan"
// @Failure      403  {object}  errors.AppError "Račun ne pripada klijentu"
// @Failure      404  {object}  errors.AppError "Kredit nije pronađen"
// @Failure      500  {object}  errors.AppError "Greška na serveru"
// @Router       /api/loans/request [post]
// @Security     BearerAuth
func (h *LoanHandler) SubmitLoanRequest(c *gin.Context) {
	var req dto.CreateLoanRequest

	// Parsiranje JSON body-ja
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.BadRequestErr("neispravan format zahteva: "+err.Error()))
		return
	}

	// Izvlačenje ClientID iz tokena pomoću AuthContext-a
	authCtx := auth.GetAuth(c)
	if authCtx == nil || authCtx.ClientID == nil {
		c.JSON(http.StatusUnauthorized, errors.UnauthorizedErr("unauthorized"))
		return
	}
	clientID := *authCtx.ClientID

	// Poziv servisa
	resp, err := h.loanService.SubmitLoanRequest(c.Request.Context(), &req, clientID)
	if err != nil {
		// Za ručno hendlovanje (ili c.Error(err) ako imate globalni middleware)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Uspešan odgovor
	c.JSON(http.StatusCreated, resp)
}

// GetLoans godoc
// @Summary      Pregled svih kredita klijenta
// @Description  Vraća listu kredita. Podržava sortiranje po iznosu.
// @Tags         loans
// @Produce      json
// @Param        sort query string false "Sortiraj po iznosu: 'asc' ili 'desc'"
// @Success      200  {array}   dto.LoanResponse
// @Router       /api/loans [get]
// @Security     BearerAuth
// @Failure      400  {object}  errors.AppError "Nevalidni podaci, valuta se ne poklapa ili los period otplate"
// @Failure      401  {object}  errors.AppError "Korisnik nije ulogovan"
// @Failure      403  {object}  errors.AppError "Račun ne pripada klijentu"
// @Failure      404  {object}  errors.AppError "Kredit nije pronađen"
// @Failure      500  {object}  errors.AppError "Greška na serveru"
func (h *LoanHandler) GetLoans(c *gin.Context) {
	authCtx := auth.GetAuth(c)
	if authCtx == nil || authCtx.ClientID == nil {
		c.JSON(http.StatusUnauthorized, errors.UnauthorizedErr("unauthorized"))
		return
	}

	sortParam := c.Query("sort")
	sortByAmountDesc := sortParam == "desc"

	loans, err := h.loanService.GetClientLoans(c.Request.Context(), *authCtx.ClientID, sortByAmountDesc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.InternalErr(err))
		return
	}

	c.JSON(http.StatusOK, loans)
}

// GetLoanByID godoc
// @Summary      Detalji kredita
// @Description  Vraća detaljne informacije o kreditu uključujući plan otplate (rate).
// @Tags         loans
// @Produce      json
// @Param        id   path      int  true  "ID kredita"
// @Success      200  {object}  dto.LoanDetailsResponse
// @Router       /api/loans/{id} [get]
// @Security     BearerAuth
// @Failure      400  {object}  errors.AppError "Nevalidni podaci, valuta se ne poklapa ili los period otplate"
// @Failure      401  {object}  errors.AppError "Korisnik nije ulogovan"
// @Failure      403  {object}  errors.AppError "Račun ne pripada klijentu"
// @Failure      404  {object}  errors.AppError "Kredit nije pronađen"
// @Failure      500  {object}  errors.AppError "Greška na serveru"
func (h *LoanHandler) GetLoanByID(c *gin.Context) {
	authCtx := auth.GetAuth(c)
	if authCtx == nil || authCtx.ClientID == nil {
		c.JSON(http.StatusUnauthorized, errors.UnauthorizedErr("unauthorized"))
		return
	}

	// Izvlačenje ID-ja iz URL-a (npr. /api/loans/5)
	idParam := c.Param("id")
	var loanID uint
	fmt.Sscanf(idParam, "%d", &loanID)

	details, err := h.loanService.GetLoanDetails(c.Request.Context(), *authCtx.ClientID, loanID)
	if err != nil {
		c.JSON(http.StatusNotFound, errors.NotFoundErr(err.Error()))
		return
	}

	c.JSON(http.StatusOK, details)
}
