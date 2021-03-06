package wf

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/yomorun/yomo/internal/conf"
	"github.com/yomorun/yomo/internal/dispatcher"
	"github.com/yomorun/yomo/internal/mocker"
	"github.com/yomorun/yomo/internal/workflow"
	"github.com/yomorun/yomo/pkg/quic"
)

// DevOptions are the options for dev command.
type DevOptions struct {
	baseOptions
}

// NewCmdDev creates a new command dev.
func NewCmdDev() *cobra.Command {
	var opts = &DevOptions{}

	var cmd = &cobra.Command{
		Use:   "dev",
		Short: "Dev a YoMo Serverless Function",
		Long:  "Dev a YoMo Serverless Function with mocking yomo-source data from YCloud.",
		Run: func(cmd *cobra.Command, args []string) {
			conf, err := parseConfig(&opts.baseOptions, args)
			if err != nil {
				log.Print("❌ ", err)
				return
			}

			log.Print("Running YoMo workflow...")
			endpoint := fmt.Sprintf("0.0.0.0:%d", conf.Port)

			quicHandler := &quicDevHandler{
				serverlessConfig: conf,
				serverAddr:       fmt.Sprintf("localhost:%d", conf.Port),
				mergeChan:        make(chan []byte, 20),
			}

			err = workflow.Run(endpoint, quicHandler)
			if err != nil {
				log.Print("❌ ", err)
				return
			}
		},
	}

	cmd.Flags().StringVarP(&opts.Config, "config", "c", "workflow.yaml", "Workflow config file (default is workflow.yaml)")

	return cmd
}

type quicDevHandler struct {
	serverlessConfig *conf.WorkflowConfig
	serverAddr       string
	mergeChan        chan []byte
}

func (s *quicDevHandler) Listen() error {
	err := mocker.EmitMockDataFromCloud(s.serverAddr)
	flows := workflow.Build(s.serverlessConfig)

	stream := dispatcher.DispatcherWithFunc(flows, s.mergeChan)

	go func() {
		for customer := range stream.Observe() {
			if customer.Error() {
				fmt.Println(customer.E.Error())
			}
		}
	}()
	return err
}

func (s *quicDevHandler) Read(st quic.Stream) error {

	go func() {
		for {
			buf := make([]byte, 3*1024)
			n, err := st.Read(buf)
			if err != nil {
				break
			} else {
				value := buf[:n]
				s.mergeChan <- value
			}
		}
	}()

	return nil
}
