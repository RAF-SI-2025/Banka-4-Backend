package permission

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"google.golang.org/grpc"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonjwt "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	perm "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
)

type fakePermissionClient struct {
	req  *pb.GetPermissionsRequest
	resp *pb.GetPermissionsResponse
	err  error
}

func (f *fakePermissionClient) GetPermissions(_ context.Context, req *pb.GetPermissionsRequest, _ ...grpc.CallOption) (*pb.GetPermissionsResponse, error) {
	f.req = req
	return f.resp, f.err
}

func TestGrpcPermissionProviderBuildsRequestAndMapsResponse(t *testing.T) {
	t.Parallel()

	employeeID := uint(33)
	client := &fakePermissionClient{resp: &pb.GetPermissionsResponse{
		Permissions: []string{string(perm.EmployeeView), string(perm.TradingMargin)},
	}}
	provider := NewGrpcPermissionProvider(client)

	got, err := provider.GetPermissions(context.Background(), &commonjwt.Claims{
		IdentityID:   15,
		IdentityType: string(commonauth.IdentityEmployee),
		EmployeeID:   &employeeID,
	})
	if err != nil {
		t.Fatalf("get permissions: %v", err)
	}

	want := []perm.Permission{perm.EmployeeView, perm.TradingMargin}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions = %#v, want %#v", got, want)
	}
	if client.req.GetIdentityId() != 15 || client.req.GetIdentityType() != string(commonauth.IdentityEmployee) {
		t.Fatalf("unexpected request identity %#v", client.req)
	}
	if client.req.GetSubjectId() != uint64(employeeID) {
		t.Fatalf("subject id = %d, want %d", client.req.GetSubjectId(), employeeID)
	}
}

func TestGrpcPermissionProviderPropagatesClientError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("permission service down")
	provider := NewGrpcPermissionProvider(&fakePermissionClient{err: wantErr})

	_, err := provider.GetPermissions(context.Background(), &commonjwt.Claims{
		IdentityID:   15,
		IdentityType: string(commonauth.IdentityClient),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected client error, got %v", err)
	}
}
