package cnsclient

import (
	"github.com/Azure/azure-container-networking/cns"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
)

// APIClient interface to update cns state
type APIClient interface {
	ReconcileNCState(nc *cns.CreateNetworkContainerRequest, pods map[string]cns.KubernetesPodInfo, scalar nnc.Scaler, spec nnc.NodeNetworkConfigSpec) error
	CreateOrUpdateNC(nc cns.CreateNetworkContainerRequest, scalar nnc.Scaler, spec nnc.NodeNetworkConfigSpec) error
}
