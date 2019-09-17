/*
 * Copyright (c) 2019-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/untillpro/godif"
)

var lastCtx context.Context

func TestBasicUsage(t *testing.T) {

	// Register services

	s1 := &MyService{Name: "Service1"}
	s2 := &MyService{Name: "Service2"}
	godif.ProvideSliceElement(&Services, s1)
	godif.ProvideSliceElement(&Services, s2)

	// Resolve and start services

	ctx, err := ResolveAndStart()
	defer StopAndReset(ctx)
	require.Nil(t, err)

	// Check service state

	assert.Equal(t, 1, s1.State)
	assert.Equal(t, 1, s2.State)

	//Make sure that value provided by service exist in ctx

	assert.Equal(t, 0, lastCtx.Value(ctxKeyType("Service1")).(int))
	assert.Equal(t, 1000, lastCtx.Value(ctxKeyType("Service2")).(int))
	assert.Nil(t, lastCtx.Value(ctxKeyType("Service3")))

	// StopServices services
	StopServices(ctx)
	assert.Equal(t, 0, s1.State)
	assert.Equal(t, 0, s2.State)
}

func TestFailedStart(t *testing.T) {

	s1 := &MyService{Name: "Service1"}
	s2 := &MyService{Name: "Service2", Failstart: true}
	godif.ProvideSliceElement(&Services, s1)
	godif.ProvideSliceElement(&Services, s2)
	Declare()

	// Resolve all

	errs := godif.ResolveAll()
	defer godif.Reset()
	assert.Nil(t, errs)
	fmt.Println("errs=", errs)

	// Start services

	var err error
	fmt.Println("### Before Start")
	ctx := context.Background()
	ctx, err = StartServices(ctx)
	defer StopServices(ctx)
	fmt.Println("### After Start")
	assert.NotNil(t, err)
	fmt.Println("err=", err)
	assert.True(t, strings.Contains(err.Error(), "Service2"))
	assert.False(t, strings.Contains(err.Error(), "Service1"))
	assert.Equal(t, 1, s1.State)
	assert.Equal(t, 0, s2.State)
}

func TestContextStartStopOrder(t *testing.T) {

	ctxKey := ctxKeyType("root")
	initialCtx := context.WithValue(context.Background(), ctxKey, "rootValue")

	prevVerbose := SetVerbose(false)
	defer SetVerbose(prevVerbose)

	var services []IService
	for i := 0; i < 100; i++ {
		s := MyService{Name: fmt.Sprint("Service", i)}
		services = append(services, &s)
	}
	finalCtx, startedServices, err := Start(initialCtx, services)
	defer Stop(finalCtx, startedServices)

	require.Equal(t, len(services), len(startedServices))
	require.Nil(t, err)

	// Check that initial context is kept
	require.Equal(t, "rootValue", finalCtx.Value(ctxKeyType("root")))

	// Check that services contexts are kept
	for idx := range startedServices {
		require.Equal(t, idx*1000, finalCtx.Value(ctxKeyType(fmt.Sprint("Service", idx))))
	}
}

func TestStartStopOrder(t *testing.T) {

	var services []*MyService

	runningServices = 0

	prevVerbose := SetVerbose(false)
	defer SetVerbose(prevVerbose)

	for i := 0; i < 100; i++ {
		s := &MyService{Name: fmt.Sprint("Service", i)}
		services = append(services, s)
		godif.ProvideSliceElement(&Services, s)
	}

	// Resolve and start services

	ctx, err := ResolveAndStart()
	defer StopAndReset(ctx)
	require.Nil(t, err)

	for i, s := range services {
		assert.Equal(t, i, s.runningServiceNumber)
	}

	StopServices(ctx)
	for _, s := range services {
		assert.Equal(t, 0, s.runningServiceNumber)
	}

}

var runningServices = 0

// MyService for testing purposes
type MyService struct {
	Name                 string
	State                int // 0 (stopped), 1 (started)
	Failstart            bool
	CtxValue             interface{}
	Wg                   *sync.WaitGroup
	runningServiceNumber int // assgined from runningServices
}

type ctxKeyType string

// Start s.e.
func (s *MyService) Start(ctx context.Context) (context.Context, error) {
	if s.Failstart {
		fmt.Println(s.Name, "Start fails")
		return ctx, errors.New(s.Name + ":" + "Start fails")
	}
	s.State++
	s.runningServiceNumber = runningServices
	runningServices++
	ctx = context.WithValue(ctx, ctxKeyType(s.Name), s.runningServiceNumber*1000)
	if nil != s.Wg {
		s.Wg.Done()
	}
	lastCtx = ctx
	return ctx, nil
}

// Stop s.e.
func (s *MyService) Stop(ctx context.Context) {
	s.State--
	runningServices--
	s.runningServiceNumber -= runningServices
}

func (s *MyService) String() string {
	return "I'm service " + s.Name
}

func Test_BasicUsage(t *testing.T) {

	var wg sync.WaitGroup
	wg.Add(2)

	// Declare two services

	s1 := &MyService{Name: "Service1", Wg: &wg}
	s2 := &MyService{Name: "Service2", Wg: &wg}
	godif.ProvideSliceElement(&Services, s1)
	godif.ProvideSliceElement(&Services, s2)

	// Terminate when all services started

	go func() {
		wg.Wait()
		Terminate()
	}()

	// Run waits for Terminate() or SIGTERM
	err := Run()
	require.Nil(t, err, err)

}

func Test_FailedStart(t *testing.T) {

	s1 := &MyService{Name: "Service1"}
	s2 := &MyService{Name: "Service2", Failstart: true}
	godif.ProvideSliceElement(&Services, s1)
	godif.ProvideSliceElement(&Services, s2)
	err := Run()
	require.NotNil(t, err, err)
	require.Equal(t, 0, s1.State)
	require.Equal(t, 0, s2.State)
}

var Missed func()

func Test_FailedResolve(t *testing.T) {
	godif.Require(&Missed)
	err := Run()
	require.NotNil(t, err, err)
}
