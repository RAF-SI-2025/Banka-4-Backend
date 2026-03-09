package service

import (
	"fmt"
)

// EmailService je prost servis koji "šalje" mejlove (za sada ispis u konzoli)
type EmailService struct{}

func NewEmailService() *EmailService {
	return &EmailService{}
}

// Send šalje mejl na konzolu
func (es *EmailService) Send(to, subject, body string) error {
	fmt.Printf("To: %s\nSubject: %s\nBody: %s\n\n", to, subject, body)
	return nil
}

// GenerateActivationLink pravi link za aktivaciju naloga
func GenerateActivationLink(email string) string {
	// za sada fiksni token, kasnije može JWT ili UUID
	return fmt.Sprintf("http://localhost:8080/activate?email=%s&token=uniquetoken", email)
}
