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

type callOptions struct {
	requestAttachment  []byte
	responseAttachment *[]byte
}

// CallOption configure a Call before it starts or extracts information from
// a Call after it completes.
type CallOption interface {
	apply(*callOptions)
}

type funcCallOption struct {
	f func(*callOptions)
}

func (fco *funcCallOption) apply(co *callOptions) {
	fco.f(co)
}

func newFuncCallOption(f func(*callOptions)) *funcCallOption {
	return &funcCallOption{
		f: f,
	}
}

func WithRequestAttachment(a []byte) CallOption {
	return newFuncCallOption(func(o *callOptions) {
		o.requestAttachment = a
	})
}

func WithResponseAttachment(a *[]byte) CallOption {
	return newFuncCallOption(func(o *callOptions) {
		o.responseAttachment = a
	})
}
