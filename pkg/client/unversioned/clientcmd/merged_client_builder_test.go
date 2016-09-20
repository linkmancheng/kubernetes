/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clientcmd

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/client/restclient"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

type testLoader struct {
	ClientConfigLoader

	called bool
	config *clientcmdapi.Config
	err    error
}

func (l *testLoader) Load() (*clientcmdapi.Config, error) {
	l.called = true
	return l.config, l.err
}

type testClientConfig struct {
	config *restclient.Config
	err    error
}

func (c *testClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, fmt.Errorf("unexpected call")
}
func (c *testClientConfig) ClientConfig() (*restclient.Config, error) {
	return c.config, c.err
}
func (c *testClientConfig) Namespace() (string, bool, error) {
	return "", false, fmt.Errorf("unexpected call")
}
func (c *testClientConfig) ConfigAccess() ConfigAccess {
	return nil
}

type testICC struct {
	testClientConfig

	possible bool
	called   bool
}

func (icc *testICC) Possible() bool {
	icc.called = true
	return icc.possible
}

func TestInClusterConfig(t *testing.T) {
	// override direct client config for this run
	originalDefault := DefaultClientConfig
	defer func() {
		DefaultClientConfig = originalDefault
	}()

	baseDefault := &DirectClientConfig{
		overrides: &ConfigOverrides{},
	}
	default1 := &DirectClientConfig{
		config:      *createValidTestConfig(),
		contextName: "clean",
		overrides:   &ConfigOverrides{},
	}
	config1, err := default1.ClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	config2 := &restclient.Config{Host: "config2"}
	err1 := fmt.Errorf("unique error")

	testCases := map[string]struct {
		clientConfig  *testClientConfig
		icc           *testICC
		defaultConfig *DirectClientConfig

		checkedICC bool
		result     *restclient.Config
		err        error
	}{
		"in-cluster checked on other error": {
			clientConfig: &testClientConfig{err: ErrEmptyConfig},
			icc:          &testICC{},

			checkedICC: true,
			result:     nil,
			err:        ErrEmptyConfig,
		},

		"in-cluster not checked on non-empty error": {
			clientConfig: &testClientConfig{err: ErrEmptyCluster},
			icc:          &testICC{},

			checkedICC: false,
			result:     nil,
			err:        ErrEmptyCluster,
		},

		"in-cluster checked when config is default": {
			defaultConfig: default1,
			clientConfig:  &testClientConfig{config: config1},
			icc:           &testICC{},

			checkedICC: true,
			result:     config1,
			err:        nil,
		},

		"in-cluster not checked when config is not equal to default": {
			defaultConfig: default1,
			clientConfig:  &testClientConfig{config: config2},
			icc:           &testICC{},

			checkedICC: false,
			result:     config2,
			err:        nil,
		},

		"in-cluster checked when config is not equal to default and error is empty": {
			clientConfig: &testClientConfig{config: config2, err: ErrEmptyConfig},
			icc:          &testICC{},

			checkedICC: true,
			result:     config2,
			err:        ErrEmptyConfig,
		},

		"in-cluster error returned when config is empty": {
			clientConfig: &testClientConfig{err: ErrEmptyConfig},
			icc: &testICC{
				possible: true,
				testClientConfig: testClientConfig{
					err: err1,
				},
			},

			checkedICC: true,
			result:     nil,
			err:        err1,
		},

		"in-cluster config returned when config is empty": {
			clientConfig: &testClientConfig{err: ErrEmptyConfig},
			icc: &testICC{
				possible: true,
				testClientConfig: testClientConfig{
					config: config2,
				},
			},

			checkedICC: true,
			result:     config2,
			err:        nil,
		},

		"in-cluster not checked when default is invalid": {
			defaultConfig: &DefaultClientConfig,
			clientConfig:  &testClientConfig{config: config2},
			icc:           &testICC{},

			checkedICC: false,
			result:     config2,
			err:        nil,
		},
	}

	for name, test := range testCases {
		if test.defaultConfig != nil {
			DefaultClientConfig = *test.defaultConfig
		} else {
			DefaultClientConfig = *baseDefault
		}
		c := &DeferredLoadingClientConfig{icc: test.icc}
		c.clientConfig = test.clientConfig

		cfg, err := c.ClientConfig()
		if test.icc.called != test.checkedICC {
			t.Errorf("%s: unexpected in-cluster-config call %t", name, test.icc.called)
		}
		if err != test.err || cfg != test.result {
			t.Errorf("%s: unexpected result: %v %#v", name, err, cfg)
		}
	}
}