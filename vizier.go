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

	"px.dev/pxapi/utils"
	"px.dev/pxapi/proto/vizierpb"
)

// VizierClient is the client for a single vizier.
type VizierClient struct {
	cloud         *Client
	useEncryption bool
	vizierID      string
	vzClient      vizierpb.VizierServiceClient
}

// ExecuteScript runs the script on vizier.
func (v *VizierClient) ExecuteScript(ctx context.Context, pxl string, mux TableMuxer) (*ScriptResults, error) {
	var encOpts, decOpts *vizierpb.ExecuteScriptRequest_EncryptionOptions
	var err error
	if v.useEncryption {
		encOpts, decOpts, err = utils.CreateEncryptionOptions()
		if err != nil {
			return nil, err
		}
	}
	req := &vizierpb.ExecuteScriptRequest{
		ClusterID:         v.vizierID,
		QueryStr:          pxl,
		EncryptionOptions: encOpts,
	}
	ctx, cancel := context.WithCancel(ctx)
	res, err := v.vzClient.ExecuteScript(v.cloud.cloudCtxWithMD(ctx), req)
	if err != nil {
		cancel()
		return nil, err
	}

	sr := newScriptResults()
	sr.c = res
	sr.cancel = cancel
	sr.tm = mux
	sr.decOpts = decOpts

	return sr, nil
}
