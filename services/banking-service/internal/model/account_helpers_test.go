package model

import "testing"

func TestIsForeignAccountNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		accountNumber string
		want          bool
	}{
		{name: "local full account", accountNumber: BankCode + "000100000000001", want: false},
		{name: "foreign full account", accountNumber: "555000100000000001", want: true},
		{name: "short foreign-looking account", accountNumber: "555", want: false},
		{name: "empty account", accountNumber: "", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsForeignAccountNumber(tt.accountNumber); got != tt.want {
				t.Fatalf("IsForeignAccountNumber(%q) = %v, want %v", tt.accountNumber, got, tt.want)
			}
		})
	}
}

func TestAccountTypeCodeHelpers(t *testing.T) {
	t.Parallel()

	if got := GetSubtypeCode(SubtypeSavings); got != "3" {
		t.Fatalf("GetSubtypeCode(Savings) = %q, want 3", got)
	}
	if got := GetSubtypeCode(Subtype("Unknown")); got != "0" {
		t.Fatalf("GetSubtypeCode(Unknown) = %q, want 0", got)
	}
	if got := GetTypeCode(AccountKindCurrent, AccountTypePersonal, SubtypeStudent); got != "16" {
		t.Fatalf("GetTypeCode(Current, Personal, Student) = %q, want 16", got)
	}
}

func TestUpdateBalances(t *testing.T) {
	t.Parallel()

	account := &Account{Balance: 100, AvailableBalance: 80}
	UpdateBalances(account, 25.5)

	if account.Balance != 125.5 {
		t.Fatalf("balance = %.2f, want 125.50", account.Balance)
	}
	if account.AvailableBalance != 105.5 {
		t.Fatalf("available balance = %.2f, want 105.50", account.AvailableBalance)
	}
}
