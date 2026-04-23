package output

import (
	"context"

	"github.com/mgoeppe/ksem/pkg/types"
)

// Handler is the interface for all output modes
type Handler interface {
	// Run starts the output handler and processes data from the channels
	Run(ctx context.Context, dataChan <-chan *types.KSEMData, errChan <-chan error) error
}
