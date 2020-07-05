// Copyright 2018-20 PJ Engineering and Business Solutions Pty. Ltd. All rights reserved.

package exports

import (
	"context"
	"io"

	"github.com/tealeg/xlsx/v3"
	dataframe "github.com/zdevwu/dataframe-go"
)

// ExcelExportOptions contains options for ExportToExcel function.
type ExcelExportOptions struct {

	// NullString is used to set what nil values should be encoded to.
	// Common options are NULL, \N, NaN, NA.
	NullString *string

	// Range is used to export a subset of rows from the Dataframe.
	Range dataframe.Range

	// WriteSheet is used to specify a sheet name.
	// When not set, it defaults to "sheet1"
	WriteSheet *string
}

// ExportToExcel exports a Dataframe to an excel file.
func ExportToExcel(ctx context.Context, w io.Writer, df *dataframe.DataFrame, options ...ExcelExportOptions) error {

	df.Lock()
	defer df.Unlock()

	var (
		sheetRow *xlsx.Row
		file     *xlsx.File
		cell     *xlsx.Cell
	)

	nullString := "NaN"    // Default value
	writeSheet := "sheet1" // Write to default sheet 1 if a different one is not set

	var r dataframe.Range

	if len(options) > 0 {
		r = options[0].Range

		if options[0].NullString != nil {
			nullString = *options[0].NullString
		}

		if options[0].WriteSheet != nil {
			writeSheet = *options[0].WriteSheet
		}
	}

	file = xlsx.NewFile()
	sheet, err := file.AddSheet(writeSheet)
	if err != nil {
		return err
	}

	// Add first row to excel sheet for header fields
	sheetRow = sheet.AddRow()
	// Write Header fields first
	for _, field := range df.Names(dataframe.DontLock) {
		cell = sheetRow.AddCell() // set column cell
		cell.Value = field        // assign field to cell
	}

	nRows := df.NRows(dataframe.DontLock)

	if nRows > 0 {

		s, e, err := r.Limits(nRows)
		if err != nil {
			return err
		}

		// Writing record in Rows
		for row := s; row <= e; row++ {

			// check if error in ctx context
			if err := ctx.Err(); err != nil {
				return err
			}

			// Add new role to excel sheet
			sheetRow = sheet.AddRow()

			// collecting rows
			for _, aSeries := range df.Series {
				val := aSeries.Value(row)
				cell = sheetRow.AddCell()
				if val == nil {
					cell.Value = nullString
				} else {
					cell.Value = aSeries.ValueString(row)
				}
			}
		}
	}

	// Save file
	err = file.Write(w)
	if err != nil {
		return err
	}

	return nil
}
