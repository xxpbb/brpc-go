/*
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
 */

package brpc

import "context"

// A Controller mediates a single method call. The primary purpose of
// the controller is to provide a way to manipulate settings per RPC-call
// and to find out about RPC-level errors.
type Controller struct {
	requestAttachment  []byte
	responseAttachment []byte
}

type controllerContextKey struct{}

var (
	gControllerContextKey = &controllerContextKey{}
)

func newContextWithController(cntl *Controller) context.Context {
	return context.WithValue(context.Background(), gControllerContextKey, cntl)
}

func GetControllerFromContext(ctx context.Context) *Controller {
	cntl, ok := ctx.Value(gControllerContextKey).(*Controller)
	if !ok {
		return nil
	}
	return cntl
}

func (c *Controller) GetRequestAttachment() []byte {
	return c.requestAttachment
}

func (c *Controller) SetResponseAttachment(a []byte) {
	c.responseAttachment = a
}
