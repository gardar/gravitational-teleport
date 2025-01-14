// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clusterconfigv1

import (
	"context"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/modules"
)

// Cache is used by the [Service] to query cluster config resources.
type Cache interface {
	GetAuthPreference(context.Context) (types.AuthPreference, error)
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
}

// Backend is used by the [Service] to mutate cluster config resources.
type Backend interface {
	CreateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	UpdateAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)
	UpsertAuthPreference(ctx context.Context, preference types.AuthPreference) (types.AuthPreference, error)

	CreateClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	UpdateClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)
	UpsertClusterNetworkingConfig(ctx context.Context, preference types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error)

	CreateSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	UpdateSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
	UpsertSessionRecordingConfig(ctx context.Context, preference types.SessionRecordingConfig) (types.SessionRecordingConfig, error)
}

// ServiceConfig contain dependencies required to create a [Service].
type ServiceConfig struct {
	Cache      Cache
	Backend    Backend
	Authorizer authz.Authorizer
}

// Service implements the teleport.clusterconfig.v1.ClusterConfigService RPC service.
type Service struct {
	clusterconfigpb.UnimplementedClusterConfigServiceServer

	cache      Cache
	backend    Backend
	authorizer authz.Authorizer
}

// NewService validates the provided configuration and returns a [Service].
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache service is required")
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend service is required")
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	}

	return &Service{cache: cfg.Cache, backend: cfg.Backend, authorizer: cfg.Authorizer}, nil
}

// GetAuthPreference returns the locally cached auth preference.
func (s *Service) GetAuthPreference(ctx context.Context, _ *clusterconfigpb.GetAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	pref, err := s.cache.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := pref.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", pref, authPrefV2))
	}

	return authPrefV2, nil
}

// CreateAuthPreference creates a new auth preference if one does not exist. This
// is an internal API and is not exposed via [clusterconfigv1.ClusterConfigServiceServer]. It
// is only meant to be called directly from the first time an Auth instance is started.
func (s *Service) CreateAuthPreference(ctx context.Context, p types.AuthPreference) (types.AuthPreference, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	// check that the given RequireMFAType is supported in this build.
	if p.GetPrivateKeyPolicy().IsHardwareKeyPolicy() && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
	}

	if err := dtconfig.ValidateConfigAgainstModules(p.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := s.backend.CreateAuthPreference(ctx, p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := created.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", created, authPrefV2))
	}

	return authPrefV2, nil
}

// UpdateAuthPreference conditionally updates an existing auth preference if the value
// wasn't concurrently modified.
func (s *Service) UpdateAuthPreference(ctx context.Context, req *clusterconfigpb.UpdateAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	// check that the given RequireMFAType is supported in this build.
	if req.AuthPreference.GetPrivateKeyPolicy().IsHardwareKeyPolicy() && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
	}

	if err := dtconfig.ValidateConfigAgainstModules(req.AuthPreference.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	req.AuthPreference.SetOrigin(types.OriginDynamic)

	updated, err := s.backend.UpdateAuthPreference(ctx, req.AuthPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := updated.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", updated, authPrefV2))
	}

	return authPrefV2, nil
}

// UpsertAuthPreference creates a new auth preference or overwrites an existing auth preference.
func (s *Service) UpsertAuthPreference(ctx context.Context, req *clusterconfigpb.UpsertAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	// check that the given RequireMFAType is supported in this build.
	if req.AuthPreference.GetPrivateKeyPolicy().IsHardwareKeyPolicy() && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("Hardware Key support is only available with an enterprise license")
	}

	if err := dtconfig.ValidateConfigAgainstModules(req.AuthPreference.GetDeviceTrust()); err != nil {
		return nil, trace.Wrap(err)
	}

	req.AuthPreference.SetOrigin(types.OriginDynamic)

	updated, err := s.backend.UpsertAuthPreference(ctx, req.AuthPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPrefV2, ok := updated.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", updated, authPrefV2))
	}

	return authPrefV2, nil
}

// ResetAuthPreference restores the auth preferences to the default value.
func (s *Service) ResetAuthPreference(ctx context.Context, _ *clusterconfigpb.ResetAuthPreferenceRequest) (*types.AuthPreferenceV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterAuthPreference, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	defaultPreference := types.DefaultAuthPreference()
	const iterationLimit = 3
	// Attempt a few iterations in case the conditional update fails
	// due to spurious networking conditions.
	for i := 0; i < iterationLimit; i++ {
		pref, err := s.cache.GetAuthPreference(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if pref.Origin() == types.OriginConfigFile {
			return nil, trace.BadParameter("auth preference has been defined via the config file and cannot be reset back to defaults dynamically.")
		}

		defaultPreference.SetRevision(pref.GetRevision())

		reset, err := s.backend.UpdateAuthPreference(ctx, defaultPreference)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		authPrefV2, ok := reset.(*types.AuthPreferenceV2)
		if !ok {
			return nil, trace.Wrap(trace.BadParameter("unexpected auth preference type %T (expected %T)", reset, authPrefV2))
		}

		return authPrefV2, nil
	}

	return nil, trace.LimitExceeded("failed to reset networking config within %v iterations", iterationLimit)
}

// GetClusterNetworkingConfig returns the locally cached networking configuration.
func (s *Service) GetClusterNetworkingConfig(ctx context.Context, _ *clusterconfigpb.GetClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbRead); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	netConfig, err := s.cache.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := netConfig.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", netConfig, cfgV2))
	}
	return cfgV2, nil
}

// CreateClusterNetworkingConfig creates a new cluster networking configuration if one does not exist.
// This is an internal API and is not exposed via [clusterconfigv1.ClusterConfigServiceServer]. It
// is only meant to be called directly from the first time an Auth instance is started.
func (s *Service) CreateClusterNetworkingConfig(ctx context.Context, cfg types.ClusterNetworkingConfig) (types.ClusterNetworkingConfig, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	tst, err := cfg.GetTunnelStrategyType()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tst == types.ProxyPeering && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	created, err := s.backend.CreateClusterNetworkingConfig(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := created.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", created, cfgV2))
	}

	return cfgV2, nil
}

// UpdateClusterNetworkingConfig conditionally updates an existing networking configuration if the
// value wasn't concurrently modified.
func (s *Service) UpdateClusterNetworkingConfig(ctx context.Context, req *clusterconfigpb.UpdateClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	tst, err := req.ClusterNetworkConfig.GetTunnelStrategyType()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tst == types.ProxyPeering && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	req.ClusterNetworkConfig.SetOrigin(types.OriginDynamic)

	updated, err := s.backend.UpdateClusterNetworkingConfig(ctx, req.ClusterNetworkConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := updated.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", updated, cfgV2))
	}

	return cfgV2, nil
}

// UpsertClusterNetworkingConfig creates a new networking configuration or overwrites an existing configuration.
func (s *Service) UpsertClusterNetworkingConfig(ctx context.Context, req *clusterconfigpb.UpsertClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbCreate, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	tst, err := req.ClusterNetworkConfig.GetTunnelStrategyType()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tst == types.ProxyPeering && modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, trace.AccessDenied("proxy peering is an enterprise-only feature")
	}

	req.ClusterNetworkConfig.SetOrigin(types.OriginDynamic)

	upserted, err := s.backend.UpsertClusterNetworkingConfig(ctx, req.GetClusterNetworkConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := upserted.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", upserted, cfgV2))
	}

	return cfgV2, nil
}

// ResetClusterNetworkingConfig restores the networking configuration to the default value.
func (s *Service) ResetClusterNetworkingConfig(ctx context.Context, _ *clusterconfigpb.ResetClusterNetworkingConfigRequest) (*types.ClusterNetworkingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindClusterNetworkingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := authzCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	defaultConfig := types.DefaultClusterNetworkingConfig()
	const iterationLimit = 3
	// Attempt a few iterations in case the conditional update fails
	// due to spurious networking conditions.
	for i := 0; i < iterationLimit; i++ {
		cfg, err := s.cache.GetClusterNetworkingConfig(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.Origin() == types.OriginConfigFile {
			return nil, trace.BadParameter("cluster networking configuration has been defined in the auth server's config file and cannot be set back to defaults dynamically.")
		}

		defaultConfig.SetRevision(cfg.GetRevision())

		reset, err := s.backend.UpdateClusterNetworkingConfig(ctx, defaultConfig)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		cfgV2, ok := reset.(*types.ClusterNetworkingConfigV2)
		if !ok {
			return nil, trace.Wrap(trace.BadParameter("unexpected cluster networking config type %T (expected %T)", reset, cfgV2))
		}

		return cfgV2, nil
	}

	return nil, trace.LimitExceeded("failed to reset networking config within %v iterations", iterationLimit)
}

// GetSessionRecordingConfig returns the locally cached networking configuration.
func (s *Service) GetSessionRecordingConfig(ctx context.Context, _ *clusterconfigpb.GetSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbRead); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbRead); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	netConfig, err := s.cache.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := netConfig.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", netConfig, cfgV2))
	}
	return cfgV2, nil
}

// CreateSessionRecordingConfig creates a new cluster networking configuration if one does not exist.
// This is an internal API and is not exposed via [clusterconfigv1.ClusterConfigServiceServer]. It
// is only meant to be called directly from the first time an Auth instance is started.
func (s *Service) CreateSessionRecordingConfig(ctx context.Context, cfg types.SessionRecordingConfig) (types.SessionRecordingConfig, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !authz.HasBuiltinRole(*authzCtx, string(types.RoleAuth)) {
		return nil, trace.AccessDenied("this request can be only executed by an auth server")
	}

	created, err := s.backend.CreateSessionRecordingConfig(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := created.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", created, cfgV2))
	}

	return cfgV2, nil
}

// UpdateSessionRecordingConfig conditionally updates an existing networking configuration if the
// value wasn't concurrently modified.
func (s *Service) UpdateSessionRecordingConfig(ctx context.Context, req *clusterconfigpb.UpdateSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	req.SessionRecordingConfig.SetOrigin(types.OriginDynamic)

	updated, err := s.backend.UpdateSessionRecordingConfig(ctx, req.SessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := updated.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", updated, cfgV2))
	}

	return cfgV2, nil
}

// UpsertSessionRecordingConfig creates a new networking configuration or overwrites an existing configuration.
func (s *Service) UpsertSessionRecordingConfig(ctx context.Context, req *clusterconfigpb.UpsertSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbCreate, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbCreate, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Support reused MFA for bulk tctl create requests.
	if err := authzCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}

	req.SessionRecordingConfig.SetOrigin(types.OriginDynamic)

	upserted, err := s.backend.UpsertSessionRecordingConfig(ctx, req.SessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfgV2, ok := upserted.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", upserted, cfgV2))
	}

	return cfgV2, nil
}

// ResetSessionRecordingConfig restores the networking configuration to the default value.
func (s *Service) ResetSessionRecordingConfig(ctx context.Context, _ *clusterconfigpb.ResetSessionRecordingConfigRequest) (*types.SessionRecordingConfigV2, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindSessionRecordingConfig, types.VerbUpdate); err != nil {
		if err2 := authzCtx.CheckAccessToKind(types.KindClusterConfig, types.VerbUpdate); err2 != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := authzCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}
	defaultConfig := types.DefaultSessionRecordingConfig()
	const iterationLimit = 3
	// Attempt a few iterations in case the conditional update fails
	// due to spurious networking conditions.
	for i := 0; i < iterationLimit; i++ {

		cfg, err := s.cache.GetSessionRecordingConfig(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if cfg.Origin() == types.OriginConfigFile {
			return nil, trace.BadParameter("session recording configuration has been defined in the auth server's config file and cannot be set back to defaults dynamically.")
		}

		defaultConfig.SetRevision(cfg.GetRevision())

		reset, err := s.backend.UpsertSessionRecordingConfig(ctx, defaultConfig)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		cfgV2, ok := reset.(*types.SessionRecordingConfigV2)
		if !ok {
			return nil, trace.Wrap(trace.BadParameter("unexpected session recording config type %T (expected %T)", reset, cfgV2))
		}

		return cfgV2, nil
	}
	return nil, trace.LimitExceeded("failed to reset networking config within %v iterations", iterationLimit)
}
