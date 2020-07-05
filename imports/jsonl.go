// Copyright 2018-20 PJ Engineering and Business Solutions Pty. Ltd. All rights reserved.

package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	dataframe "github.com/zdevwu/dataframe-go"
)

// JSONLoadOptions is likely to change.
type JSONLoadOptions struct {

	// LargeDataSet should be set to true for large datasets.
	// It will set the capacity of the underlying slices of the Dataframe by performing a basic parse
	// of the full dataset before processing the data fully.
	// Preallocating memory can provide speed improvements. Benchmarks should be performed for your use-case.
	LargeDataSet bool

	// DictateDataType is used to inform LoadFromJSON what the true underlying data type is for a given field name.
	// The key must be the case-sensitive field name.
	// The value for a given key must be of the data type of the data.
	// eg. For a string use "". For a int64 use int64(0). What is relevant is the data type and not the value itself.
	//
	// NOTE: A custom Series must implement NewSerieser interface and be able to interpret strings to work.
	DictateDataType map[string]interface{}

	// ErrorOnUnknownFields will generate an error if an unknown field is encountered after the first row.
	ErrorOnUnknownFields bool
}

// LoadFromJSON will load data from a jsonl file.
// The first row determines which fields will be imported for subsequent rows.
func LoadFromJSON(ctx context.Context, r io.ReadSeeker, options ...JSONLoadOptions) (*dataframe.DataFrame, error) {

	var init *dataframe.SeriesInit

	if len(options) > 0 {
		// Count how many rows we have in order to preallocate underlying slices
		if options[0].LargeDataSet {
			init = &dataframe.SeriesInit{}
			dec := json.NewDecoder(r)

			tokenCount := 0
			for {
				if err := ctx.Err(); err != nil {
					return nil, err
				}

				t, err := dec.Token()
				if err != nil {
					if err == io.EOF {
						r.Seek(0, io.SeekStart)
						break
					}
					return nil, err
				}

				switch delim := t.(type) {
				case json.Delim:
					if delim.String() == "{" {
						tokenCount++
					} else if delim.String() == "}" {
						tokenCount--
						if tokenCount == 0 {
							init.Size++
						}
					}
				}
			}
		}
	}

	knownFields := map[string]interface{}{} // These fields are determined by the first row

	var row int
	var df *dataframe.DataFrame

	dec := json.NewDecoder(r)
	dec.UseNumber()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var raw map[string]interface{}
		err := dec.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		row++

		vals := parseObject(raw, "")

		if row == 1 {

			// The first row determines which fields we use
			knownFields = vals

			// Create a series for each field (of the appropriate data type)
			seriess := []dataframe.Series{}

			for name := range vals {

				// Check if we know what the datatype should be. Otherwise assume string
				if len(options) > 0 && len(options[0].DictateDataType) > 0 {

					typ, exists := options[0].DictateDataType[name]
					if !exists {
						seriess = append(seriess, dataframe.NewSeriesString(name, init))
						continue
					}

					switch T := typ.(type) {
					case float64:
						seriess = append(seriess, dataframe.NewSeriesFloat64(name, init))
					case int64, bool:
						seriess = append(seriess, dataframe.NewSeriesInt64(name, init))
					case string:
						seriess = append(seriess, dataframe.NewSeriesString(name, init))
					case time.Time:
						seriess = append(seriess, dataframe.NewSeriesTime(name, init))
					case dataframe.NewSerieser:
						seriess = append(seriess, T.NewSeries(name, init))
					case Converter:
						switch T.ConcreteType.(type) {
						case time.Time:
							seriess = append(seriess, dataframe.NewSeriesTime(name, init))
						default:
							seriess = append(seriess, dataframe.NewSeriesGeneric(name, T.ConcreteType, init))
						}
					default:
						seriess = append(seriess, dataframe.NewSeriesGeneric(name, typ, init))
					}
				} else {
					seriess = append(seriess, dataframe.NewSeriesString(name, init))
				}

			}

			// Create the dataframe
			df = dataframe.NewDataFrame(seriess...)

			// Store values of first row into dataframe
			insertVals := map[string]interface{}{}

			for name, val := range vals {

				// Store values
				if len(options) > 0 && len(options[0].DictateDataType) > 0 {

					// Check if a datatype is dictated
					typ, exists := options[0].DictateDataType[name]
					if !exists {
						// Store value as a string
						switch v := val.(type) {
						case string:
							insertVals[name] = v
						case json.Number:
							insertVals[name] = v.String()
						case bool:
							if v == true {
								insertVals[name] = "1"
							} else {
								insertVals[name] = "0"
							}
						}
					} else {
						err := dictateForce(row, insertVals, name, typ, val)
						if err != nil {
							return nil, err
						}
					}
				} else {
					// Store value as a string

					switch v := val.(type) {
					case string:
						insertVals[name] = v
					case json.Number:
						insertVals[name] = v.String()
					case bool:
						if v == true {
							insertVals[name] = "1"
						} else {
							insertVals[name] = "0"
						}
					}
				}

			}

			if init == nil {
				df.Append(&dataframe.DontLock, make([]interface{}, len(df.Series))...)
			}
			df.UpdateRow(row-1, &dataframe.DontLock, insertVals)

		} else {

			insertVals := map[string]interface{}{}

			for name, val := range vals {

				// Check if field is a known field
				_, exists := knownFields[name]
				if !exists {
					// unknown field
					if len(options) > 0 && options[0].ErrorOnUnknownFields {
						return nil, fmt.Errorf("unknown field encountered. row: %d field: %s", row-1, name)
					}
					continue
				}

				// Store values
				if len(options) > 0 && len(options[0].DictateDataType) > 0 {

					// Check if a datatype is dictated
					typ, exists := options[0].DictateDataType[name]
					if !exists {
						// Store value as a string
						switch v := val.(type) {
						case string:
							insertVals[name] = v
						case json.Number:
							insertVals[name] = v.String()
						case bool:
							if v == true {
								insertVals[name] = "1"
							} else {
								insertVals[name] = "0"
							}
						}
					} else {
						err := dictateForce(row, insertVals, name, typ, val)
						if err != nil {
							return nil, err
						}
					}
				} else {
					// Store value as a string

					switch v := val.(type) {
					case string:
						insertVals[name] = v
					case json.Number:
						insertVals[name] = v.String()
					case bool:
						if v == true {
							insertVals[name] = "1"
						} else {
							insertVals[name] = "0"
						}
					}
				}
			}

			if init == nil {
				df.Append(&dataframe.DontLock, make([]interface{}, len(df.Series))...)
			}
			df.UpdateRow(row-1, &dataframe.DontLock, insertVals)

		}
	}

	if df == nil {
		return nil, dataframe.ErrNoRows
	}

	// The order is not stable
	names := df.Names(dataframe.DontLock)
	sort.Strings(names)
	df.ReorderColumns(names)

	return df, nil
}
