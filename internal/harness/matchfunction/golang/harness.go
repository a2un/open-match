// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package harness

import (
	"fmt"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"google.golang.org/grpc"
	"open-match.dev/open-match/internal/config"
	"open-match.dev/open-match/internal/harness/matchfunction/golang/apisrv"
	"open-match.dev/open-match/internal/logging"
	"open-match.dev/open-match/internal/metrics"
	"open-match.dev/open-match/internal/pb"
	"open-match.dev/open-match/internal/serving"
	"open-match.dev/open-match/internal/signal"
	"open-match.dev/open-match/internal/util/netlistener"

	"github.com/sirupsen/logrus"
)

// Params is a collection of parameters used to create a MatchFunction server.
type Params struct {
	FunctionName          string
	ServicePortConfigName string
	ProxyPortConfigName   string
	Func                  apisrv.MatchFunction
}

// ServeMatchFunction is a hook for the main() method in the main executable.
func ServeMatchFunction(params *Params) {
	mfServer, err := newMatchFunctionServer(params)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Info("Cannot construct the match function server.")
		return
	}

	// Instantiate the gRPC server with the bindings we've made.
	logger := mfServer.Logger
	grpcLh, err := netlistener.NewFromPortNumber(mfServer.Config.GetInt(params.ServicePortConfigName))
	proxyLh, err := netlistener.NewFromPortNumber(mfServer.Config.GetInt(params.ProxyPortConfigName))
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Fatal("Failed to create a TCP listener for the GRPC server")
		return
	}

	grpcServer := serving.NewGrpcServer(grpcLh, proxyLh, logger)
	grpcServer.AddService(func(server *grpc.Server) {
		pb.RegisterMatchFunctionServer(server, mfServer)
	})
	grpcServer.AddProxy(pb.RegisterMatchFunctionHandler)

	defer func() {
		err := grpcServer.Stop()
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err.Error()}).Infof("Server shutdown error, %s.", err)
		}
	}()

	// Start serving traffic.
	err = grpcServer.Start()
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Fatal("Failed to start server")
	}

	// Exit when we see a signal
	wait, _ := signal.New()
	wait()
	logger.Info("Shutting down server")
}

// newMatchFunctionServer creates a MatchFunctionServer based on the harness parameters.
func newMatchFunctionServer(params *Params) (*apisrv.MatchFunctionServer, error) {
	logrus.AddHook(metrics.NewHook(apisrv.HarnessLogLines, apisrv.KeySeverity))
	logger := logrus.WithFields(logrus.Fields{
		"app":       "openmatch",
		"component": "matchfunction_service",
		"function":  params.FunctionName})

	// Add a hook to the logger to log the filename & line number.
	logrus.SetReportCaller(true)

	cfg, err := config.Read()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Unable to load config file")
		return nil, err
	}

	// Configure open match logging defaults
	logging.ConfigureLogging(cfg)

	// Configure OpenCensus exporter to Prometheus
	// metrics.ConfigureOpenCensusPrometheusExporter expects that every OpenCensus view you
	// want to register is in an array, so append any views you want from other
	// packages to a single array here.
	ocServerViews := []*view.View{}
	ocServerViews = append(ocServerViews, apisrv.DefaultFunctionViews...)
	ocServerViews = append(ocServerViews, ocgrpc.DefaultServerViews...) // gRPC OpenCensus views.
	ocServerViews = append(ocServerViews, config.CfgVarCountView)       // config loader view.
	logger.WithFields(logrus.Fields{"viewscount": len(ocServerViews)}).Info("Loaded OpenCensus views")

	promLh, err := netlistener.NewFromPortNumber(cfg.GetInt("metrics.port"))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Unable to create metrics TCP listener")
		return nil, err
	}
	metrics.ConfigureOpenCensusPrometheusExporter(promLh, cfg, ocServerViews)

	var mmlogic pb.MmLogicClient
	mmlogic, err = getMMLogicClient(cfg)
	if err != nil {
		logger.Errorf("Failed to get MMLogic client, %v.", err)
		return nil, err
	}

	mfServer := &apisrv.MatchFunctionServer{
		FunctionName: params.FunctionName,
		Logger:       logger,
		Config:       cfg,
		Func:         params.Func,
		MMLogic:      mmlogic,
	}

	return mfServer, nil
}

func getMMLogicClient(cfg config.View) (pb.MmLogicClient, error) {
	host := cfg.GetString("api.mmlogic.hostname")
	if len(host) == 0 {
		return nil, fmt.Errorf("Failed to get hostname for MMLogicAPI from the configuration")
	}

	port := cfg.GetString("api.mmlogic.port")
	if len(port) == 0 {
		return nil, fmt.Errorf("Failed to get port for MMLogicAPI from the configuration")
	}

	conn, err := grpc.Dial(fmt.Sprintf("%v:%v", host, port), grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %v, %v", fmt.Sprintf("%v:%v", host, port), err)
	}

	return pb.NewMmLogicClient(conn), nil
}
