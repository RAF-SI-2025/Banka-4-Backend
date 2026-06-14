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

	clientID := uint(77)
	client := &fakePermissionClient{resp: &pb.GetPermissionsResponse{
		Permissions: []string{string(perm.ClientView), string(perm.Trading)},
	}}
	provider := NewGrpcPermissionProvider(client)

	got, err := provider.GetPermissions(context.Background(), &commonjwt.Claims{
		IdentityID:   12,
		IdentityType: string(commonauth.IdentityClient),
		ClientID:     &clientID,
	})
	if err != nil {
		t.Fatalf("get permissions: %v", err)
	}

	want := []perm.Permission{perm.ClientView, perm.Trading}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions = %#v, want %#v", got, want)
	}
	if client.req.GetIdentityId() != 12 || client.req.GetIdentityType() != string(commonauth.IdentityClient) {
		t.Fatalf("unexpected request identity %#v", client.req)
	}
	if client.req.GetSubjectId() != uint64(clientID) {
		t.Fatalf("subject id = %d, want %d", client.req.GetSubjectId(), clientID)
	}
}

func TestGrpcPermissionProviderPropagatesClientError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("permission service down")
	provider := NewGrpcPermissionProvider(&fakePermissionClient{err: wantErr})

	_, err := provider.GetPermissions(context.Background(), &commonjwt.Claims{
		IdentityID:   12,
		IdentityType: string(commonauth.IdentityEmployee),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected client error, got %v", err)
	}
}
