package grpc

import (
	"context"

	pkgerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
)

type UserService struct {
	pb.UnimplementedUserServiceServer
	clientRepo     repository.ClientRepository
	employeeRepo   repository.EmployeeRepository
	clientService  *service.ClientService
	actuaryService *service.ActuaryService
}

func NewUserService(clientRepo repository.ClientRepository, employeeRepo repository.EmployeeRepository, clientService *service.ClientService,
	actuaryService *service.ActuaryService) *UserService {
	return &UserService{clientRepo: clientRepo, employeeRepo: employeeRepo, clientService: clientService,
		actuaryService: actuaryService}
}

func (s *UserService) GetClientById(ctx context.Context, req *pb.GetClientByIdRequest) (*pb.GetClientByIdResponse, error) {
	client, err := s.clientRepo.FindByID(ctx, uint(req.Id))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch client: %v", err)
	}
	if client == nil {
		return nil, status.Errorf(codes.NotFound, "client %d not found", req.Id)
	}
	return &pb.GetClientByIdResponse{
		Id:         uint64(client.ClientID),
		Email:      client.Identity.Email,
		FullName:   client.FirstName + " " + client.LastName,
		IdentityId: uint64(client.IdentityID),
	}, nil
}

func (s *UserService) GetClientsByIds(
	ctx context.Context,
	req *pb.GetClientsByIdsRequest,
) (*pb.GetClientsByIdsResponse, error) {

	ids := make([]uint, 0, len(req.Ids))
	for _, id := range req.Ids {
		ids = append(ids, uint(id))
	}

	clients, err := s.clientRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch clients: %v", err)
	}

	if len(clients) == 0 {
		return nil, status.Errorf(codes.NotFound, "clients not found")
	}

	resp := &pb.GetClientsByIdsResponse{
		Clients: make([]*pb.GetClientByIdResponse, 0, len(clients)),
	}

	for _, client := range clients {
		resp.Clients = append(resp.Clients, &pb.GetClientByIdResponse{
			Id:         uint64(client.ClientID),
			Email:      client.Identity.Email,
			FullName:   client.FirstName + " " + client.LastName,
			IdentityId: uint64(client.IdentityID),
		})
	}

	return resp, nil
}

func (s *UserService) GetEmployeeById(ctx context.Context, req *pb.GetEmployeeByIdRequest) (*pb.GetEmployeeByIdResponse, error) {
	employee, err := s.employeeRepo.FindByID(ctx, uint(req.Id))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch employee: %v", err)
	}
	if employee == nil {
		return nil, status.Errorf(codes.NotFound, "employee %d not found", req.Id)
	}
	resp := &pb.GetEmployeeByIdResponse{
		Id:           uint64(employee.EmployeeID),
		Email:        employee.Identity.Email,
		FullName:     employee.FirstName + " " + employee.LastName,
		IsSupervisor: employee.IsSupervisor(),
		IsAgent:      employee.IsAgent(),
		IdentityId:   uint64(employee.IdentityID),
	}

	if employee.ActuaryInfo != nil {
		resp.NeedApproval = employee.ActuaryInfo.NeedApproval
		resp.OrderLimit = employee.ActuaryInfo.Limit
		resp.UsedLimit = employee.ActuaryInfo.UsedLimit
	}

	return resp, nil
}
func (s *UserService) GetClientByIdentityID(ctx context.Context, req *pb.GetClientByIdentityIdRequest) (*pb.GetClientByIdResponse, error) {
	client, err := s.clientRepo.FindByIdentityID(ctx, uint(req.IdentityId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch client: %v", err)
	}
	if client == nil {
		return nil, status.Errorf(codes.NotFound, "client with user_id %d not found", req.IdentityId)
	}
	return &pb.GetClientByIdResponse{
		Id:         uint64(client.ClientID),
		Email:      client.Identity.Email,
		FullName:   client.FirstName + " " + client.LastName,
		IdentityId: uint64(client.IdentityID),
	}, nil
}

func (s *UserService) GetEmployeeByIdentityID(ctx context.Context, req *pb.GetEmployeeByIdentityIdRequest) (*pb.GetEmployeeByIdResponse, error) {
	employee, err := s.employeeRepo.FindByIdentityID(ctx, uint(req.IdentityId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch employee: %v", err)
	}
	if employee == nil {
		return nil, status.Errorf(codes.NotFound, "employee with user_id %d not found", req.IdentityId)
	}
	resp := &pb.GetEmployeeByIdResponse{
		Id:           uint64(employee.EmployeeID),
		Email:        employee.Identity.Email,
		FullName:     employee.FirstName + " " + employee.LastName,
		IsSupervisor: employee.IsSupervisor(),
		IsAgent:      employee.IsAgent(),
		IdentityId:   uint64(employee.IdentityID),
	}
	if employee.ActuaryInfo != nil {
		resp.NeedApproval = employee.ActuaryInfo.NeedApproval
		resp.OrderLimit = employee.ActuaryInfo.Limit
		resp.UsedLimit = employee.ActuaryInfo.UsedLimit
	}
	return resp, nil
}

func (s *UserService) GetAllClients(ctx context.Context, req *pb.GetAllClientsRequest) (*pb.GetAllClientsResponse, error) {
	query := &dto.ListClientsQuery{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Page:      int(req.Page),
		PageSize:  int(req.PageSize),
	}

	result, err := s.clientService.GetAllClients(ctx, query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch clients: %v", err)
	}

	pbClients := make([]*pb.ClientResponse, len(result.Data))
	for i, c := range result.Data {
		pbClients[i] = &pb.ClientResponse{
			Id:        uint64(c.Id),
			FirstName: c.FirstName,
			LastName:  c.LastName,
			Email:     c.Email,
		}
	}

	return &pb.GetAllClientsResponse{
		Clients:    pbClients,
		Total:      result.Total,
		Page:       int32(result.Page),
		PageSize:   int32(result.PageSize),
		TotalPages: int32(result.TotalPages),
	}, nil
}

func (s *UserService) GetAllActuaries(ctx context.Context, req *pb.GetAllActuariesRequest) (*pb.GetAllActuariesResponse, error) {
	query := &dto.ListActuariesQuery{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Page:      int(req.Page),
		PageSize:  int(req.PageSize),
	}

	result, err := s.actuaryService.GetAllActuaries(ctx, query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch actuaries: %v", err)
	}

	pbActuaries := make([]*pb.ActuaryResponse, len(result.Data))
	for i, a := range result.Data {
		pbActuaries[i] = &pb.ActuaryResponse{
			Id:        uint64(a.ID),
			FirstName: a.FirstName,
			LastName:  a.LastName,
			Email:     a.Email,
		}
	}

	return &pb.GetAllActuariesResponse{
		Actuaries:  pbActuaries,
		Total:      result.Total,
		Page:       int32(result.Page),
		PageSize:   int32(result.PageSize),
		TotalPages: int32(result.TotalPages),
	}, nil
}

func (s *UserService) GetIdentityByUserId(ctx context.Context, req *pb.GetIdentityByUserIdRequest) (*pb.GetIdentityByUserIdResponse, error) {
	var identityID uint

	switch userType := req.UserType; userType {
	case "ACTUARY":
		employee, err := s.employeeRepo.FindByID(ctx, uint(req.UserId))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to fetch employee: %v", err)
		}
		if employee == nil {
			return nil, status.Errorf(codes.NotFound, "employee %d not found", req.UserId)
		}
		identityID = employee.Identity.ID
	case "CLIENT":
		client, err := s.clientRepo.FindByID(ctx, uint(req.UserId))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to fetch client: %v", err)
		}
		if client == nil {
			return nil, status.Errorf(codes.NotFound, "client %d not found", req.UserId)
		}
		identityID = client.Identity.ID
	default:
		return nil, status.Errorf(codes.Internal, "wrong user type: %s", userType)

	}

	resp := &pb.GetIdentityByUserIdResponse{
		IdentityId: uint64(identityID),
	}

	return resp, nil
}

func (s *UserService) IncrementUsedLimit(ctx context.Context, req *pb.IncrementUsedLimitRequest) (*pb.IncrementUsedLimitResponse, error) {
	usedLimit, err := s.actuaryService.IncrementUsedLimit(ctx, uint(req.EmployeeId), req.Amount)
	if err != nil {
		return nil, pkgerrors.MapGrpcToHttpError(err)
	}

	return &pb.IncrementUsedLimitResponse{UsedLimit: usedLimit}, nil
}
