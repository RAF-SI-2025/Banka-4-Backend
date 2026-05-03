package dto

type CreateCompanyRequest struct {
	Name               string `json:"name" binding:"required"`
	RegistrationNumber string `json:"registration_number" binding:"required,max=8"`
	TaxNumber          string `json:"tax_number" binding:"required,max=9"`
	WorkCodeID         uint   `json:"work_code_id" binding:"required"`
	Address            string `json:"address"`
	OwnerID            uint   `json:"owner_id" binding:"required"`
}
