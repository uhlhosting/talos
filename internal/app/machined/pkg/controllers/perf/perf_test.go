// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package perf_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/perf"
	"github.com/talos-systems/talos/pkg/logging"
	perfresource "github.com/talos-systems/talos/pkg/machinery/resources/perf"
)

type PerfSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *PerfSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	logger := logging.Wrap(log.Writer())

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)
}

func (suite *PerfSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *PerfSuite) TestReconcile() {
	suite.Require().NoError(suite.runtime.RegisterController(&perf.StatsController{}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			cpu, err := suite.state.Get(suite.ctx, resource.NewMetadata(perfresource.NamespaceName, perfresource.CPUType, perfresource.CPUID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			mem, err := suite.state.Get(suite.ctx, resource.NewMetadata(perfresource.NamespaceName, perfresource.MemoryType, perfresource.MemoryID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			cpuSpec := cpu.Spec().(*perfresource.CPUSpec)    //nolint:errcheck,forcetypeassert
			memSpec := mem.Spec().(*perfresource.MemorySpec) //nolint:errcheck,forcetypeassert

			if len(cpuSpec.CPU) == 0 || memSpec.MemTotal == 0 {
				return retry.ExpectedError(fmt.Errorf("cpu spec does not contain any CPU or Total memory is zero"))
			}

			return nil
		},
	))
}

func (suite *PerfSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestPerfSuite(t *testing.T) {
	suite.Run(t, new(PerfSuite))
}
