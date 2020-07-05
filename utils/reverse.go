// Copyright 2019-20 PJ Engineering and Business Solutions Pty. Ltd. All rights reserved.

package utils

import (
	"context"

	dataframe "github.com/zdevwu/dataframe-go"
)

type common interface {
	Lock()
	Unlock()
	NRows(options ...dataframe.Options) int
	Swap(row1, row2 int, options ...dataframe.Options)
}

// ReverseOptions modifies the behavior of Reverse.
type ReverseOptions struct {

	// R is used to limit the range of the Series for search purposes.
	R *dataframe.Range

	// DontLock can be set to true if the Series should not be locked.
	DontLock bool
}

// Reverse will reverse the order of a Dataframe or Series.
// If a Range is provided, only the rows within the range are reversed.
// s will be locked for the duration of the operation.
func Reverse(ctx context.Context, sdf common, opts ...ReverseOptions) error {

	if len(opts) == 0 {
		opts = append(opts, ReverseOptions{R: &dataframe.Range{}})
	} else if opts[0].R == nil {
		opts[0].R = &dataframe.Range{}
	}

	if !opts[0].DontLock {
		sdf.Lock()
		defer sdf.Unlock()
	}

	nRows := sdf.NRows(dataframe.DontLock)
	if nRows == 0 {
		return nil
	}

	start, _, err := opts[0].R.Limits(nRows)
	if err != nil {
		return err
	}

	rRows, _ := opts[0].R.NRows(nRows)

	if rRows == 1 || rRows == 0 {
		return nil
	}

	for i := rRows/2 - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		opp := rRows - 1 - i
		sdf.Swap(i+start, opp+start, dataframe.DontLock)
	}

	return nil
}
