package wdp

import (
	"math"

	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
)

func init() {
	beam.RegisterFunction(extractPage)
}

func CountPageViews(s beam.Scope, col beam.PCollection) beam.PCollection {
	s = s.Scope("countPageViews")
	pageviews := beam.ParDo(s, extractPage, col)
	viewsPerPage := stats.Count(s, pageviews)
	return viewsPerPage
}

func PrivateCountPageViews(s beam.Scope, col beam.PCollection, epsilon float64, sensitivity int) beam.PCollection {
	s = s.Scope("countPageViews")

	spec := pbeam.NewPrivacySpec(epsilon, sensitivity)
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "id")

	pageviews := pbeam.ParDo(s, extractPage, pCol)
	viewsPerPage := pbeam.Count(s, pageviews, pbeam.CountParams{
		MaxPartitionsContributed:	1, // In the scheme I've constructed, each visitor visits once per day
		MaxValues: 					1, // And they can visit a maximum of one page
	})
	return viewsPerPage
}

func extractPage(p PageView) string {
	return p.Name
}