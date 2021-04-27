/*
 * Copyright 2018- The Pixie Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package pxapi

import (
	"context"

	"px.dev/pxapi/errdefs"
	"px.dev/pxapi/proto/cloudapipb"
)

// VizierStatus stores the enumeration of all vizier statuses.
type VizierStatus string

// Vizier Statuses.
const (
	VizierStatusUnknown      VizierStatus = "Unknown"
	VizierStatusHealthy      VizierStatus = "Healthy"
	VizierStatusUnhealthy    VizierStatus = "Unhealthy"
	VizierStatusDisconnected VizierStatus = "Disconnected"
)

// VizierInfo has information of a single Vizier.
type VizierInfo struct {
	// Name of the vizier.
	Name string
	// ID of the Vizier (uuid as a string).
	ID string
	// Status of the Vizier.
	Status VizierStatus
	// Version of the installed vizier.
	Version string
	// DirectAccess says the cluster has direct access mode enabled. This means the data transfer will bypass the cloud.
	DirectAccess bool
}

func clusterStatusToVizierStatus(status cloudapipb.ClusterStatus) VizierStatus {
	switch status {
	case cloudapipb.CS_HEALTHY:
		return VizierStatusHealthy
	case cloudapipb.CS_UNHEALTHY:
		return VizierStatusUnhealthy
	case cloudapipb.CS_DISCONNECTED:
		return VizierStatusDisconnected
	default:
		return VizierStatusUnknown
	}
}

// ListViziers gets a list of Viziers registered with Pixie.
func (c *Client) ListViziers(ctx context.Context) ([]*VizierInfo, error) {
	req := &cloudapipb.GetClusterInfoRequest{}
	res, err := c.cmClient.GetClusterInfo(c.cloudCtxWithMD(ctx), req)
	if err != nil {
		return nil, err
	}

	viziers := make([]*VizierInfo, 0)
	for _, v := range res.Clusters {
		viziers = append(viziers, &VizierInfo{
			Name:         v.ClusterName,
			ID:           ProtoToUUIDStr(v.ID),
			Version:      v.VizierVersion,
			Status:       clusterStatusToVizierStatus(v.Status),
			DirectAccess: !v.Config.PassthroughEnabled,
		})
	}

	return viziers, nil
}

// GetVizierInfo gets info about the given clusterID.
func (c *Client) GetVizierInfo(ctx context.Context, clusterID string) (*VizierInfo, error) {
	req := &cloudapipb.GetClusterInfoRequest{
		ID: ProtoFromUUIDStrOrNil(clusterID),
	}
	res, err := c.cmClient.GetClusterInfo(c.cloudCtxWithMD(ctx), req)
	if err != nil {
		return nil, err
	}

	if len(res.Clusters) == 0 {
		return nil, errdefs.ErrClusterNotFound
	}

	v := res.Clusters[0]

	return &VizierInfo{
		Name:         v.ClusterName,
		ID:           ProtoToUUIDStr(v.ID),
		Version:      v.VizierVersion,
		Status:       clusterStatusToVizierStatus(v.Status),
		DirectAccess: !v.Config.PassthroughEnabled,
	}, nil
}

// getConnectionInfo gets the connection info for a cluster using direct mode.
func (c *Client) getConnectionInfo(ctx context.Context, clusterID string) (*cloudapipb.GetClusterConnectionInfoResponse, error) {
	req := &cloudapipb.GetClusterConnectionInfoRequest{
		ID: ProtoFromUUIDStrOrNil(clusterID),
	}
	return c.cmClient.GetClusterConnectionInfo(c.cloudCtxWithMD(ctx), req)
}

// CreateDeployKey creates a new deploy key, with an optional description.
func (c *Client) CreateDeployKey(ctx context.Context, desc string) (*cloudapipb.DeploymentKey, error) {
	keyMgr := cloudapipb.NewVizierDeploymentKeyManagerClient(c.grpcConn)
	req := &cloudapipb.CreateDeploymentKeyRequest{
		Desc: desc,
	}
	dk, err := keyMgr.Create(c.cloudCtxWithMD(ctx), req)
	if err != nil {
		return nil, err
	}
	return dk, nil
}
