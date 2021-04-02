package updater

import (
	"context"

	"github.com/spraints/mytagdata/pkg/data"
)

type Updater interface {
	Update(context.Context, data.WirelessTagUpdate) error
}

type MultiUpdater struct {
	Updaters []Updater
}

func (m *MultiUpdater) Update(ctx context.Context, u data.WirelessTagUpdate) error {
	for _, updater := range m.Updaters {
		if err := updater.Update(ctx, u); err != nil {
			return err
		}
	}
	return nil
}
