package health

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Check() string {
	return "ok"
}
