package wdp

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apache/beam/sdks/go/pkg/beam"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*PageView)(nil)))
	beam.RegisterFunction(CreatePageViewsFn)
}

type PageView struct {
	ID 		string
	Name 	string
}

func CreatePageViewsFn(line string, emit func(PageView)) error {
	// Skip the column headers line
	notHeader, err := regexp.MatchString("[0-9]", line)
	if err != nil {
		return err
	}
	if !notHeader {
		return nil
	}

	cols := strings.Split(line, ",")
	if len(cols) != 2 {
		return fmt.Errorf("got %d number of columns in line %q, expected 2", len(cols), line)
	}
	id := cols[0]
	name := cols[1]
	emit(Visit{
		ID:		id,
		Name: 	name,
	})
	return nil
}
