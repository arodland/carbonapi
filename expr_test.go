package main

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/davecgh/go-spew/spew"
	pb "github.com/dgryski/carbonzipper/carbonzipperpb"
)

func TestGetBuckets(t *testing.T) {
	tests := []struct {
		start       int32
		stop        int32
		bucketSize  int32
		wantBuckets int32
	}{
		{13, 18, 5, 1},
		{13, 17, 5, 1},
		{13, 19, 5, 2},
	}

	for _, test := range tests {
		buckets := getBuckets(test.start, test.stop, test.bucketSize)
		if buckets != test.wantBuckets {
			t.Errorf("TestGetBuckets failed!\n%v\ngot buckets %d",
				test,
				buckets,
			)
		}
	}
}

func TestAlignToBucketSize(t *testing.T) {
	tests := []struct {
		inputStart int32
		inputStop  int32
		bucketSize int32
		wantStart  int32
		wantStop   int32
	}{
		{
			13, 18, 5,
			10, 20,
		},
		{
			13, 17, 5,
			10, 20,
		},
		{
			13, 19, 5,
			10, 20,
		},
	}

	for _, test := range tests {
		start, stop := alignToBucketSize(test.inputStart, test.inputStop, test.bucketSize)
		if start != test.wantStart || stop != test.wantStop {
			t.Errorf("TestAlignToBucketSize failed!\n%v\ngot start %d stop %d",
				test,
				start,
				stop,
			)
		}
	}
}

func TestAlignToInterval(t *testing.T) {
	tests := []struct {
		inputStart int32
		inputStop  int32
		bucketSize int32
		wantStart  int32
	}{
		{
			91111, 92222, 5,
			91111,
		},
		{
			91111, 92222, 60,
			91080,
		},
		{
			91111, 92222, 3600,
			90000,
		},
		{
			91111, 92222, 86400,
			86400,
		},
	}

	for _, test := range tests {
		start := alignStartToInterval(test.inputStart, test.inputStop, test.bucketSize)
		if start != test.wantStart {
			t.Errorf("TestAlignToInterval failed!\n%v\ngot start %d",
				test,
				start,
			)
		}
	}
}

func TestEvalExpr(t *testing.T) {
	exp, _, err := parseExpr("summarize(metric1,'1min')")
	if err != nil {
		t.Errorf("error %s", err)
	}

	metricMap := make(map[metricRequest][]*metricData)
	request := metricRequest{
		metric: "metric1",
		from:   1437127020,
		until:  1437127140,
	}

	stepTime := int32(60)

	data := metricData{
		FetchResponse: pb.FetchResponse{
			Name:      &request.metric,
			StartTime: &request.from,
			StopTime:  &request.until,
			StepTime:  &stepTime,
			Values:    []float64{343, 407, 385},
			IsAbsent:  []bool{false, false, false},
		},
	}

	metricMap[request] = []*metricData{
		&data,
	}

	evalExpr(exp, int32(request.from), int32(request.until), metricMap)
}

func TestParseExpr(t *testing.T) {

	tests := []struct {
		s string
		e *expr
	}{
		{"metric",
			&expr{target: "metric"},
		},
		{
			"metric.foo",
			&expr{target: "metric.foo"},
		},
		{"metric.*.foo",
			&expr{target: "metric.*.foo"},
		},
		{
			"func(metric)",
			&expr{
				target:    "func",
				etype:     etFunc,
				args:      []*expr{&expr{target: "metric"}},
				argString: "metric",
			},
		},
		{
			"func(metric1,metric2,metric3)",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"}},
				argString: "metric1,metric2,metric3",
			},
		},
		{
			"func1(metric1,func2(metricA, metricB),metric3)",
			&expr{
				target: "func1",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "func2",
						etype:     etFunc,
						args:      []*expr{&expr{target: "metricA"}, &expr{target: "metricB"}},
						argString: "metricA, metricB",
					},
					&expr{target: "metric3"}},
				argString: "metric1,func2(metricA, metricB),metric3",
			},
		},

		{
			"3",
			&expr{val: 3, etype: etConst},
		},
		{
			"3.1",
			&expr{val: 3.1, etype: etConst},
		},
		{
			"func1(metric1, 3, 1e2, 2e-3)",
			&expr{
				target: "func1",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 3, etype: etConst},
					&expr{val: 100, etype: etConst},
					&expr{val: 0.002, etype: etConst},
				},
				argString: "metric1, 3, 1e2, 2e-3",
			},
		},
		{
			"func1(metric1, 'stringconst')",
			&expr{
				target: "func1",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "stringconst", etype: etString},
				},
				argString: "metric1, 'stringconst'",
			},
		},
		{
			`func1(metric1, "stringconst")`,
			&expr{
				target: "func1",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "stringconst", etype: etString},
				},
				argString: `metric1, "stringconst"`,
			},
		},
		{
			"func1(metric1, -3)",
			&expr{
				target: "func1",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: -3, etype: etConst},
				},
				argString: "metric1, -3",
			},
		},

		{
			"func(metric, key='value')",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
				},
				namedArgs: map[string]*expr{
					"key": &expr{etype: etString, valStr: "value"},
				},
				argString: "metric, key='value'",
			},
		},
		{
			"func(metric, key=true)",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
				},
				namedArgs: map[string]*expr{
					"key": &expr{etype: etName, target: "true"},
				},
				argString: "metric, key=true",
			},
		},
		{
			"func(metric, key=1)",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
				},
				namedArgs: map[string]*expr{
					"key": &expr{etype: etConst, val: 1},
				},
				argString: "metric, key=1",
			},
		},
		{
			"func(metric, key=0.1)",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
				},
				namedArgs: map[string]*expr{
					"key": &expr{etype: etConst, val: 0.1},
				},
				argString: "metric, key=0.1",
			},
		},

		{
			"func(metric, 1, key='value')",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
					&expr{etype: etConst, val: 1},
				},
				namedArgs: map[string]*expr{
					"key": &expr{etype: etString, valStr: "value"},
				},
				argString: "metric, 1, key='value'",
			},
		},
		{
			"func(metric, key='value', 1)",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
					&expr{etype: etConst, val: 1},
				},
				namedArgs: map[string]*expr{
					"key": &expr{etype: etString, valStr: "value"},
				},
				argString: "metric, key='value', 1",
			},
		},
		{
			"func(metric, key1='value1', key2='value2')",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
				},
				namedArgs: map[string]*expr{
					"key1": &expr{etype: etString, valStr: "value1"},
					"key2": &expr{etype: etString, valStr: "value2"},
				},
				argString: "metric, key1='value1', key2='value2'",
			},
		},
		{
			"func(metric, key2='value2', key1='value1')",
			&expr{
				target: "func",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric"},
				},
				namedArgs: map[string]*expr{
					"key2": &expr{etype: etString, valStr: "value2"},
					"key1": &expr{etype: etString, valStr: "value1"},
				},
				argString: "metric, key2='value2', key1='value1'",
			},
		},

		{
			`foo.{bar,baz}.qux`,
			&expr{
				target: "foo.{bar,baz}.qux",
				etype:  etName,
			},
		},
		{
			`foo.b[0-9].qux`,
			&expr{
				target: "foo.b[0-9].qux",
				etype:  etName,
			},
		},
	}

	for _, tt := range tests {
		e, _, err := parseExpr(tt.s)
		if err != nil {
			t.Errorf("parse for %+v failed: err=%v", tt.s, err)
			continue
		}
		if !reflect.DeepEqual(e, tt.e) {
			t.Errorf("parse for %+v failed:\ngot  %+s\nwant %+v", tt.s, spew.Sdump(e), spew.Sdump(tt.e))
		}
	}
}

func makeResponse(name string, values []float64, step, start int32) *metricData {

	absent := make([]bool, len(values))

	for i, v := range values {
		if math.IsNaN(v) {
			values[i] = 0
			absent[i] = true
		}
	}

	stop := start + int32(len(values))*step

	return &metricData{FetchResponse: pb.FetchResponse{
		Name:      proto.String(name),
		Values:    values,
		StartTime: proto.Int32(start),
		StepTime:  proto.Int32(step),
		StopTime:  proto.Int32(stop),
		IsAbsent:  absent,
	}}
}

func TestEvalExpression(t *testing.T) {

	now32 := int32(time.Now().Unix())

	tests := []struct {
		e    *expr
		m    map[metricRequest][]*metricData
		want []*metricData
	}{
		{
			&expr{target: "metric"},
			map[metricRequest][]*metricData{
				metricRequest{"metric", 0, 1}: []*metricData{makeResponse("metric", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("metric", []float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "sum",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"}},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, 3, 4, 5, math.NaN()}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, 3, math.NaN(), 5, 6, math.NaN()}, 1, now32)},
				metricRequest{"metric3", 0, 1}: []*metricData{makeResponse("metric3", []float64{3, 4, 5, 6, math.NaN(), math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("sumSeries(metric1,metric2,metric3)", []float64{6, 9, 8, 15, 11, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "percentileOfSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 4, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("percentileOfSeries(metric1,4)", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "percentileOfSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.*.*"},
					&expr{val: 50, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.*.*", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, math.NaN(), math.NaN()}, 1, now32),
					makeResponse("metric1.foo.bar1.qux", []float64{6, 7, 8, 9, 10, math.NaN()}, 1, now32),
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15, math.NaN()}, 1, now32),
					makeResponse("metric1.foo.bar2.qux", []float64{7, 8, 9, 10, 11, math.NaN()}, 1, now32),
				},
			},
			[]*metricData{makeResponse("percentileOfSeries(metric1.foo.*.*,50)", []float64{7, 8, 9, 10, 11, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "percentileOfSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.*.*"},
					&expr{val: 50, etype: etConst},
				},
				namedArgs: map[string]*expr{
					"interpolate": &expr{target: "true", etype: etName},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.*.*", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, math.NaN(), math.NaN()}, 1, now32),
					makeResponse("metric1.foo.bar1.qux", []float64{6, 7, 8, 9, 10, math.NaN()}, 1, now32),
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15, math.NaN()}, 1, now32),
					makeResponse("metric1.foo.bar2.qux", []float64{7, 8, 9, 10, 11, math.NaN()}, 1, now32),
				},
			},
			[]*metricData{makeResponse("percentileOfSeries(metric1.foo.*.*,50,interpolate=true)", []float64{6.5, 7.5, 8.5, 9.5, 11, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "nPercentile",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 50, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{2, 4, 6, 10, 14, 20, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("nPercentile(metric1,50)", []float64{8, 8, 8, 8, 8, 8, 8}, 1, now32)},
		},
		{
			&expr{
				target: "nonNegativeDerivative",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{2, 4, 6, 10, 14, 20}, 1, now32)},
			},
			[]*metricData{makeResponse("nonNegativeDerivative(metric1)", []float64{math.NaN(), 2, 2, 4, 4, 6}, 1, now32)},
		},
		{
			&expr{
				target: "nonNegativeDerivative",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{2, 4, 6, 1, 4, math.NaN(), 8}, 1, now32)},
			},
			[]*metricData{makeResponse("nonNegativeDerivative(metric1)", []float64{math.NaN(), 2, 2, math.NaN(), 3, math.NaN(), math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "nonNegativeDerivative",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
				namedArgs: map[string]*expr{
					"maxValue": &expr{val: 32, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{2, 4, 0, 10, 1, math.NaN(), 8, 40, 37}, 1, now32)},
			},
			[]*metricData{makeResponse("nonNegativeDerivative(metric1,maxValue=32)", []float64{math.NaN(), 2, 29, 10, 24, math.NaN(), math.NaN(), 32, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "perSecond",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{27, 19, math.NaN(), 10, 1, 100, 1.5, 10.20}, 1, now32)},
			},
			[]*metricData{makeResponse("perSecond(metric1)", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 99, math.NaN(), 8.7}, 1, now32)},
		},
		{
			&expr{
				target: "perSecond",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 32, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{math.NaN(), 1, 2, 3, 4, 30, 0, 32, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("perSecond(metric1,32)", []float64{math.NaN(), math.NaN(), 1, 1, 1, 26, 3, 32, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "movingAverage",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 4, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8}, 1, now32)},
			},
			[]*metricData{makeResponse("movingAverage(metric1,4)", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), 1, 1.25, 1.5, 1.75, 2.5, 3.5, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "movingMedian",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 4, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8}, 1, now32)},
			},
			[]*metricData{makeResponse("movingMedian(metric1,4)", []float64{math.NaN(), math.NaN(), math.NaN(), 1, 1, 1.5, 2, 2, 3, 4, 5, 6}, 1, now32)},
		},
		{
			&expr{
				target: "movingMedian",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8, 1, 2, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("movingMedian(metric1,5)", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), 1, 1, 2, 2, 2, 4, 4, 6, 6, 4, 2}, 1, now32)},
		},
		{
			&expr{
				target: "movingMedian",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "1s", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8, 1, 2, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("movingMedian(metric1,'1s')", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8, 1, 2, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "movingMedian",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "1min", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 2, 2, 2, 4, 6, 4, 6, 8, 1, 2, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("movingMedian(metric1,'1min')", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "pearson",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{val: 6, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{43, 21, 25, 42, 57, 59}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{99, 65, 79, 75, 87, 81}, 1, now32)},
			},
			[]*metricData{makeResponse("pearson(metric1,metric2,6)", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 0.5298089018901744}, 1, now32)},
		},
		{
			&expr{
				target: "scale",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 2.5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, math.NaN(), 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("scale(metric1,2.5)", []float64{2.5, 5.0, math.NaN(), 10.0, 12.5}, 1, now32)},
		},
		{
			&expr{
				target: "scaleToSeconds",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{60, 120, math.NaN(), 120, 120}, 60, now32)},
			},
			[]*metricData{makeResponse("scaleToSeconds(metric1,5)", []float64{5, 10, math.NaN(), 10, 10}, 1, now32)},
		},
		{
			&expr{
				target: "pow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 3, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{5, 1, math.NaN(), 0, 12, 125, 10.4, 1.1}, 60, now32)},
			},
			[]*metricData{makeResponse("pow(metric1,3)", []float64{125, 1, math.NaN(), 0, 1728, 1953125, 1124.864, 1.331}, 1, now32)},
		},
		{
			&expr{
				target: "keepLastValue",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
				namedArgs: map[string]*expr{
					"limit": &expr{val: 3, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(), 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("keepLastValue(metric1,limit=3)", []float64{math.NaN(), 2, 2, 2, 2, math.NaN(), 4, 5}, 1, now32)},
		},

		{
			&expr{
				target: "keepLastValue",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(), 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("keepLastValue(metric1)", []float64{math.NaN(), 2, 2, 2, 2, 2, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "changed",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), 0, 0, 0, math.NaN(), math.NaN(), 1, 1, 2, 3, 4, 4, 5, 5, 5, 6, 7}, 1, now32)},
			},
			[]*metricData{makeResponse("changed(metric1)",
				[]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 1, 1, 0, 1, 0, 0, 1, 1}, 1, now32)},
		},
		{
			&expr{
				target: "alias",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "renamed", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("renamed",
				[]float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "aliasByMetric",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.bar.baz"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.bar.baz", 0, 1}: []*metricData{makeResponse("metric1.foo.bar.baz", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("baz", []float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "aliasByNode",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.bar.baz"},
					&expr{val: 1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.bar.baz", 0, 1}: []*metricData{makeResponse("metric1.foo.bar.baz", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("foo", []float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "aliasByNode",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.bar.baz"},
					&expr{val: 1, etype: etConst},
					&expr{val: 3, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.bar.baz", 0, 1}: []*metricData{makeResponse("metric1.foo.bar.baz", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("foo.baz",
				[]float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "aliasByNode",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.bar.baz"},
					&expr{val: 1, etype: etConst},
					&expr{val: -2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.bar.baz", 0, 1}: []*metricData{makeResponse("metric1.foo.bar.baz", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("foo.bar",
				[]float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "aliasSub",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.bar.baz"},
					&expr{valStr: "foo", etype: etString},
					&expr{valStr: "replaced", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.bar.baz", 0, 1}: []*metricData{makeResponse("metric1.foo.bar.baz", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("metric1.replaced.bar.baz",
				[]float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "aliasSub",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.TCP100"},
					&expr{valStr: "^.*TCP(\\d+)", etype: etString},
					&expr{valStr: "$1", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.TCP100", 0, 1}: []*metricData{makeResponse("metric1.TCP100", []float64{1, 2, 3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("100",
				[]float64{1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "derivative",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{2, 4, 6, 1, 4, math.NaN(), 8}, 1, now32)},
			},
			[]*metricData{makeResponse("derivative(metric1)",
				[]float64{math.NaN(), 2, 2, -5, 3, math.NaN(), 4}, 1, now32)},
		},
		{
			&expr{
				target: "avg",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"}},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), 2, 3, 4, 5}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 5, 6}, 1, now32)},
				metricRequest{"metric3", 0, 1}: []*metricData{makeResponse("metric3", []float64{3, math.NaN(), 4, 5, 6, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("averageSeries(metric1,metric2,metric3)",
				[]float64{2, math.NaN(), 3, 4, 5, 5.5}, 1, now32)},
		},
		{
			&expr{
				target: "maxSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"}},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), 2, 3, 4, 5}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 5, 6}, 1, now32)},
				metricRequest{"metric3", 0, 1}: []*metricData{makeResponse("metric3", []float64{3, math.NaN(), 4, 5, 6, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("maxSeries(metric1,metric2,metric3)",
				[]float64{3, math.NaN(), 4, 5, 6, 6}, 1, now32)},
		},
		{
			&expr{
				target: "minSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"}},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), 2, 3, 4, 5}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 5, 6}, 1, now32)},
				metricRequest{"metric3", 0, 1}: []*metricData{makeResponse("metric3", []float64{3, math.NaN(), 4, 5, 6, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("minSeries(metric1,metric2,metric3)",
				[]float64{1, math.NaN(), 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "asPercent",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32)},
			},
			[]*metricData{makeResponse("asPercent(metric1,metric2)",
				[]float64{50, math.NaN(), math.NaN(), math.NaN(), math.NaN(), 200}, 1, now32)},
		},
		{
			&expr{
				target: "divideSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32)},
			},
			[]*metricData{makeResponse("divideSeries(metric1,metric2)",
				[]float64{0.5, math.NaN(), math.NaN(), math.NaN(), math.NaN(), 2}, 1, now32)},
		},
		{
			&expr{
				target: "divideSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric[12]"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric[12]", 0, 1}: []*metricData{
					makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32),
					makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("divideSeries(metric[12])",
				[]float64{0.5, math.NaN(), math.NaN(), math.NaN(), math.NaN(), 2}, 1, now32)},
		},
		{
			&expr{
				target: "multiplySeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32)},
			},
			[]*metricData{makeResponse("multiplySeries(metric1,metric2)",
				[]float64{2, math.NaN(), math.NaN(), math.NaN(), 0, 72}, 1, now32)},
		},
		{
			&expr{
				target: "multiplySeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric[12]"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric[12]", 0, 1}: []*metricData{
					makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32),
					makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("multiplySeries(metric[12])",
				[]float64{2, math.NaN(), math.NaN(), math.NaN(), 0, 72}, 1, now32)},
		},
		{
			&expr{
				target: "multiplySeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32)},
				metricRequest{"metric3", 0, 1}: []*metricData{makeResponse("metric3", []float64{3, math.NaN(), 4, math.NaN(), 7, 8}, 1, now32)},
			},
			[]*metricData{makeResponse("multiplySeries(metric1,metric2,metric3)",
				[]float64{6, math.NaN(), math.NaN(), math.NaN(), 0, 576}, 1, now32)},
		},
		{
			&expr{
				target: "diffSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32)},
			},
			[]*metricData{makeResponse("diffSeries(metric1,metric2)",
				[]float64{-1, math.NaN(), math.NaN(), 3, 4, 6}, 1, now32)},
		},
		{
			&expr{
				target: "diffSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{target: "metric3"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{5, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
				metricRequest{"metric2", 0, 1}: []*metricData{makeResponse("metric2", []float64{3, math.NaN(), 3, math.NaN(), 0, 7}, 1, now32)},
				metricRequest{"metric3", 0, 1}: []*metricData{makeResponse("metric3", []float64{1, math.NaN(), 3, math.NaN(), 0, 4}, 1, now32)},
			},
			[]*metricData{makeResponse("diffSeries(metric1,metric2,metric3)",
				[]float64{1, math.NaN(), math.NaN(), 3, 4, 1}, 1, now32)},
		},
		{
			&expr{
				target: "diffSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32),
					makeResponse("metric2", []float64{2, math.NaN(), 3, math.NaN(), 0, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("diffSeries(metric*)",
				[]float64{-1, math.NaN(), math.NaN(), 3, 4, 6}, 1, now32)},
		},
		{
			&expr{
				target: "transformNull",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
			},
			[]*metricData{makeResponse("transformNull(metric1)",
				[]float64{1, 0, 0, 3, 4, 12}, 1, now32)},
		},
		{
			&expr{
				target: "transformNull",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
				namedArgs: map[string]*expr{
					"default": &expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, math.NaN(), math.NaN(), 3, 4, 12}, 1, now32)},
			},
			[]*metricData{makeResponse("transformNull(metric1,default=5)",
				[]float64{1, 5, 5, 3, 4, 12}, 1, now32)},
		},
		{
			&expr{
				target: "highestMax",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{1, 1, 3, 3, 12, 11}, 1, now32),
					makeResponse("metricB", []float64{1, 1, 3, 3, 4, 1}, 1, now32),
					makeResponse("metricC", []float64{1, 1, 3, 3, 4, 10}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricA", // NOTE(dgryski): not sure if this matches graphite
				[]float64{1, 1, 3, 3, 12, 11}, 1, now32)},
		},
		{
			&expr{
				target: "lowestCurrent",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32),
					makeResponse("metricB", []float64{1, 1, 3, 3, 4, 1}, 1, now32),
					makeResponse("metricC", []float64{1, 1, 3, 3, 4, 15}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricB", // NOTE(dgryski): not sure if this matches graphite
				[]float64{1, 1, 3, 3, 4, 1}, 1, now32)},
		},
		{
			&expr{
				target: "highestCurrent",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metric0", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 1, now32),
					makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32),
					makeResponse("metricB", []float64{1, 1, 3, 3, 4, 1}, 1, now32),
					makeResponse("metricC", []float64{1, 1, 3, 3, 4, 15}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricC", // NOTE(dgryski): not sure if this matches graphite
				[]float64{1, 1, 3, 3, 4, 15}, 1, now32)},
		},
		{
			&expr{
				target: "highestAverage",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32),
					makeResponse("metricB", []float64{1, 5, 5, 5, 5, 5}, 1, now32),
					makeResponse("metricC", []float64{1, 1, 3, 3, 4, 10}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricB", // NOTE(dgryski): not sure if this matches graphite
				[]float64{1, 5, 5, 5, 5, 5}, 1, now32)},
		},
		{
			&expr{
				target: "exclude",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "(Foo|Baz)", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricFoo", []float64{1, 1, 1, 1, 1}, 1, now32),
					makeResponse("metricBar", []float64{2, 2, 2, 2, 2}, 1, now32),
					makeResponse("metricBaz", []float64{3, 3, 3, 3, 3}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricBar", // NOTE(dgryski): not sure if this matches graphite
				[]float64{2, 2, 2, 2, 2}, 1, now32)},
		},
		{
			&expr{
				target: "ewma",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 0.1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{0, 1, 1, 1, math.NaN(), 1, 1}, 1, now32)},
			},
			[]*metricData{
				makeResponse("ewma(metric1,0.1)", []float64{0, 0.9, 0.99, 0.999, math.NaN(), 0.9999, 0.99999}, 1, now32),
			},
		},
		{
			&expr{
				target: "exponentialWeightedMovingAverage",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 0.1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{0, 1, 1, 1, math.NaN(), 1, 1}, 1, now32)},
			},
			[]*metricData{
				makeResponse("ewma(metric1,0.1)", []float64{0, 0.9, 0.99, 0.999, math.NaN(), 0.9999, 0.99999}, 1, now32),
			},
		},
		{
			&expr{
				target: "grep",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "Bar", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricFoo", []float64{1, 1, 1, 1, 1}, 1, now32),
					makeResponse("metricBar", []float64{2, 2, 2, 2, 2}, 1, now32),
					makeResponse("metricBaz", []float64{3, 3, 3, 3, 3}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricBar", // NOTE(dgryski): not sure if this matches graphite
				[]float64{2, 2, 2, 2, 2}, 1, now32)},
		},
		{
			&expr{
				target: "logarithm",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 10, 100, 1000, 10000}, 1, now32)},
			},
			[]*metricData{makeResponse("logarithm(metric1)",
				[]float64{0, 1, 2, 3, 4}, 1, now32)},
		},
		{
			&expr{
				target: "logarithm",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
				namedArgs: map[string]*expr{
					"base": &expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, 4, 8, 16, 32}, 1, now32)},
			},
			[]*metricData{makeResponse("logarithm(metric1,base=2)",
				[]float64{0, 1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "absolute",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{0, -1, 2, -3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("absolute(metric1)",
				[]float64{0, 1, 2, 3, 4, 5}, 1, now32)},
		},
		{
			&expr{
				target: "isNonNull",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{math.NaN(), -1, math.NaN(), -3, 4, 5}, 1, now32)},
			},
			[]*metricData{makeResponse("isNonNull(metric1)",
				[]float64{0, 1, 0, 1, 1, 1}, 1, now32)},
		},
		{
			&expr{
				target: "averageAbove",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
				makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
			},
		},
		{
			&expr{
				target: "averageBelow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 0, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{0, 4, 4, 5, 5, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricA",
				[]float64{0, 0, 0, 0, 0, 0}, 1, now32)},
		},
		{
			&expr{
				target: "maximumAbove",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 6, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricB",
				[]float64{3, 4, 5, 6, 7, 8}, 1, now32)},
		},
		{
			&expr{
				target: "maximumBelow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricA",
				[]float64{0, 0, 0, 0, 0, 0}, 1, now32)},
		},
		{
			&expr{
				target: "minimumAbove",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 1, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{1, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{2, 4, 4, 5, 5, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricC",
				[]float64{2, 4, 4, 5, 5, 6}, 1, now32)},
		},
		{
			&expr{
				target: "minimumBelow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: -2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{-1, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{-2, 4, 4, 5, 5, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricC",
				[]float64{-2, 4, 4, 5, 5, 6}, 1, now32)},
		},
		{
			&expr{
				target: "pearsonClosest",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{target: "metric2"},
					&expr{val: 1, etype: etConst},
				},
				namedArgs: map[string]*expr{
					"direction": &expr{valStr: "abs", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricX", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
				},
				metricRequest{"metric2", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, math.NaN(), 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricB",
				[]float64{3, math.NaN(), 5, 6, 7, 8}, 1, now32)},
		},
		{
			&expr{
				target: "invert",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{-4, -2, -1, 0, 1, 2, 4}, 1, now32)},
			},
			[]*metricData{makeResponse("invert(metric1)",
				[]float64{-0.25, -0.5, -1, math.NaN(), 1, 0.5, 0.25}, 1, now32)},
		},
		{
			&expr{
				target: "offset",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 10, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{93, 94, 95, math.NaN(), 97, 98, 99, 100, 101}, 1, now32)},
			},
			[]*metricData{makeResponse("offset(metric1,10)",
				[]float64{103, 104, 105, math.NaN(), 107, 108, 109, 110, 111}, 1, now32)},
		},
		{
			&expr{
				target: "offsetToZero",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{93, 94, 95, math.NaN(), 97, 98, 99, 100, 101}, 1, now32)},
			},
			[]*metricData{makeResponse("offsetToZero(metric1)",
				[]float64{0, 1, 2, math.NaN(), 4, 5, 6, 7, 8}, 1, now32)},
		},
		{
			&expr{
				target: "currentAbove",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 7, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricB",
				[]float64{3, 4, 5, 6, 7, 8}, 1, now32)},
		},
		{
			&expr{
				target: "currentBelow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 0, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, math.NaN()}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{0, 4, 4, 5, 5, 6}, 1, now32),
				},
			},
			[]*metricData{makeResponse("metricA",
				[]float64{0, 0, 0, 0, 0, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "integral",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 2, 3, 4, 5, math.NaN(), 7, 8}, 1, now32)},
			},
			[]*metricData{makeResponse("integral(metric1)",
				[]float64{1, 1, 3, 6, 10, 15, math.NaN(), 22, 30}, 1, now32)},
		},
		{
			&expr{
				target: "sortByTotal",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{5, 5, 5, 5, 5, 5}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 4, 4}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metricB", []float64{5, 5, 5, 5, 5, 5}, 1, now32),
				makeResponse("metricC", []float64{4, 4, 5, 5, 4, 4}, 1, now32),
				makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
			},
		},
		{
			&expr{
				target: "sortByMaxima",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{5, 5, 5, 5, 5, 5}, 1, now32),
					makeResponse("metricC", []float64{2, 2, 10, 5, 2, 2}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metricC", []float64{2, 2, 10, 5, 2, 2}, 1, now32),
				makeResponse("metricB", []float64{5, 5, 5, 5, 5, 5}, 1, now32),
				makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
			},
		},
		{
			&expr{
				target: "sortByMinima",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
				makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
				makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
			},
		},
		{
			&expr{
				target: "sortByName",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricX", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricA", []float64{0, 1, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{0, 0, 2, 0, 0, 0}, 1, now32),
					makeResponse("metricC", []float64{0, 0, 0, 3, 0, 0}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metricA", []float64{0, 1, 0, 0, 0, 0}, 1, now32),
				makeResponse("metricB", []float64{0, 0, 2, 0, 0, 0}, 1, now32),
				makeResponse("metricC", []float64{0, 0, 0, 3, 0, 0}, 1, now32),
				makeResponse("metricX", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
			},
		},
		{
			&expr{
				target: "sortByName",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
				namedArgs: map[string]*expr{
					"natural": &expr{target: "true", etype: etName},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metric1", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metric12", []float64{0, 1, 0, 0, 0, 0}, 1, now32),
					makeResponse("metric1234567890", []float64{0, 0, 0, 5, 0, 0}, 1, now32),
					makeResponse("metric2", []float64{0, 0, 2, 0, 0, 0}, 1, now32),
					makeResponse("metric11", []float64{0, 0, 0, 3, 0, 0}, 1, now32),
					makeResponse("metric", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metric", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
				makeResponse("metric1", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
				makeResponse("metric2", []float64{0, 0, 2, 0, 0, 0}, 1, now32),
				makeResponse("metric11", []float64{0, 0, 0, 3, 0, 0}, 1, now32),
				makeResponse("metric12", []float64{0, 1, 0, 0, 0, 0}, 1, now32),
				makeResponse("metric1234567890", []float64{0, 0, 0, 5, 0, 0}, 1, now32),
			},
		},
		{
			&expr{
				target: "constantLine",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"42.42", 0, 1}: []*metricData{makeResponse("constantLine", []float64{12.3, 12.3}, 1, now32)},
			},
			[]*metricData{makeResponse("42.42",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "threshold",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{},
			[]*metricData{makeResponse("42.42",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "threshold",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
					&expr{valStr: "fourty-two", etype: etString},
				},
			},
			map[metricRequest][]*metricData{},
			[]*metricData{makeResponse("fourty-two",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "threshold",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
					&expr{valStr: "fourty-two", etype: etString},
					&expr{valStr: "blue", etype: etString},
				},
			},
			map[metricRequest][]*metricData{},
			[]*metricData{makeResponse("fourty-two",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "threshold",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
				},
				namedArgs: map[string]*expr{
					"label": &expr{valStr: "fourty-two", etype: etString},
				},
			},
			map[metricRequest][]*metricData{},
			[]*metricData{makeResponse("fourty-two",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "threshold",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
				},
				namedArgs: map[string]*expr{
					"color": &expr{valStr: "blue", etype: etString},
					//TODO(nnuss): test blue is being set rather than just not causing expression to parse/fail
				},
			},
			map[metricRequest][]*metricData{},
			[]*metricData{makeResponse("42.42",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "threshold",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 42.42, etype: etConst},
				},
				namedArgs: map[string]*expr{
					"label": &expr{valStr: "fourty-two-blue", etype: etString},
					"color": &expr{valStr: "blue", etype: etString},
					//TODO(nnuss): test blue is being set rather than just not causing expression to parse/fail
				},
			},
			map[metricRequest][]*metricData{},
			[]*metricData{makeResponse("fourty-two-blue",
				[]float64{42.42, 42.42}, 1, now32)},
		},
		{
			&expr{
				target: "squareRoot",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, 0, 7, 8, 20, 30, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("squareRoot(metric1)",
				[]float64{1, 1.4142135623730951, 0, 2.6457513110645907, 2.8284271247461903, 4.47213595499958, 5.477225575051661, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "removeEmptySeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32),
					makeResponse("metric2", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 1, now32),
					makeResponse("metric3", []float64{0, 0, 0, 0, 0, 0, 0, 0}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32),
				makeResponse("metric3", []float64{0, 0, 0, 0, 0, 0, 0, 0}, 1, now32),
			},
		},
		{
			&expr{
				target: "removeZeroSeries",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32),
					makeResponse("metric2", []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 1, now32),
					makeResponse("metric3", []float64{0, 0, 0, 0, 0, 0, 0, 0}, 1, now32),
				},
			},
			[]*metricData{
				makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32),
			},
		},
		{
			&expr{
				target: "removeBelowValue",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 0, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("removeBelowValue(metric1,0)",
				[]float64{1, 2, math.NaN(), 7, 8, 20, 30, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "removeAboveValue",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 10, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("removeAboveValue(metric1,10)",
				[]float64{1, 2, -1, 7, 8, math.NaN(), math.NaN(), math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "removeBelowPercentile",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 50, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("removeBelowPercentile(metric1,50)",
				[]float64{math.NaN(), math.NaN(), math.NaN(), 7, 8, 20, 30, math.NaN()}, 1, now32)},
		},
		{
			&expr{
				target: "removeAbovePercentile",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 50, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 2, -1, 7, 8, 20, 30, math.NaN()}, 1, now32)},
			},
			[]*metricData{makeResponse("removeAbovePercentile(metric1,50)",
				[]float64{1, 2, -1, 7, math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 1, now32)},
		},
	}

	for _, tt := range tests {
		oldValues := map[metricRequest][]*metricData{}
		for key, metrics := range tt.m {
			entry := []*metricData{}
			for _, value := range metrics {
				newValue := metricData{
					FetchResponse: pb.FetchResponse{
						Name:      value.Name,
						StartTime: value.StartTime,
						StopTime:  value.StopTime,
						StepTime:  value.StepTime,
						Values:    make([]float64, len(value.Values)),
						IsAbsent:  make([]bool, len(value.IsAbsent)),
					},
				}

				copy(newValue.Values, value.Values)
				copy(newValue.IsAbsent, value.IsAbsent)
				entry = append(entry, &newValue)
			}

			oldValues[key] = entry
		}

		fillArgString(tt.e)
		g, err := evalExpr(tt.e, 0, 1, tt.m)
		if err != nil {
			t.Errorf("failed to eval %s: %s", tt.e.GetName(), err)
			continue
		}
		if len(g) != len(tt.want) {
			t.Errorf("%s returned a different number of metrics, actual %v, want %v", tt.e.GetName(), len(g), len(tt.want))
			continue
		}
		for key, metrics := range tt.m {
			for i, newValue := range metrics {
				oldValue := oldValues[key][i]
				if !reflect.DeepEqual(oldValue, newValue) {
					t.Errorf("%s: source data was modified key %v index %v want:\n%v\n got:\n%v", tt.e.GetName(), key, i, oldValue, newValue)
				}
			}
		}

		for i, want := range tt.want {
			actual := g[i]
			if actual == nil {
				t.Errorf("returned no value %v", tt.e.GetName())
				continue
			}
			if actual.GetStepTime() == 0 {
				t.Errorf("missing step for %+v", g)
			}
			if actual.GetName() != want.GetName() {
				t.Errorf("bad name for %s metric %d: got %s, want %s", tt.e.GetName(), i, actual.GetName(), want.GetName())
			}
			if !nearlyEqualMetrics(actual, want) {
				t.Errorf("different values for %s metric %s: got %v, want %v", tt.e.GetName(), actual.GetName(), actual.Values, want.Values)
				continue
			}
		}
	}
}

func (e expr) toString() string {
	switch e.etype {
	case etName:
		return e.target
	case etConst:
		return strconv.FormatFloat(e.val, 'f', -1, 64)
	}
	return fmt.Sprintf("'%s'", e.valStr)
}

func fillArgString(exp *expr) {
	if exp.argString != "" {
		return
	}
	metricTargets := make([]string, len(exp.args)+len(exp.namedArgs))
	for i, e := range exp.args {
		metricTargets[i] = e.toString()
	}

	i := len(exp.args)
	for key, value := range exp.namedArgs {
		metricTargets[i] = fmt.Sprintf("%s=%s", key, value.toString())
		i++
	}

	exp.argString = strings.Join(metricTargets, ",")
}

func TestEvalSummarize(t *testing.T) {

	t0, err := time.Parse(time.UnixDate, "Wed Sep 10 10:32:00 CEST 2014")
	if err != nil {
		panic(err)
	}

	tenThirtyTwo := int32(t0.Unix())

	t0, err = time.Parse(time.UnixDate, "Wed Sep 10 10:59:00 CEST 2014")
	if err != nil {
		panic(err)
	}

	tenFiftyNine := int32(t0.Unix())

	t0, err = time.Parse(time.UnixDate, "Wed Sep 10 10:30:00 CEST 2014")
	if err != nil {
		panic(err)
	}

	tenThirty := int32(t0.Unix())

	now32 := tenThirty

	tests := []struct {
		e     *expr
		m     map[metricRequest][]*metricData
		w     []float64
		name  string
		step  int32
		start int32
		stop  int32
	}{
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1,
					2, 2, 2, 2, 2,
					3, 3, 3, 3, 3,
					4, 4, 4, 4, 4,
					5, 5, 5, 5, 5,
					math.NaN(), 2, 3, 4, 5,
					math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				}, 1, now32)},
			},
			[]float64{5, 10, 15, 20, 25, 14, math.NaN()},
			"summarize(metric1,'5s')",
			5,
			now32,
			now32 + 35,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
				},
				namedArgs: map[string]*expr{
					"func": &expr{valStr: "avg", etype: etString},
				},
				argString: "metric1,'5s','avg'",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 1, 2, 3, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}, 1, now32)},
			},
			[]float64{1, 2, 3, 4, 5, 2, math.NaN()},
			"summarize(metric1,'5s','avg')",
			5,
			now32,
			now32 + 35,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
				},
				namedArgs: map[string]*expr{
					"func": &expr{valStr: "max", etype: etString},
				},
				argString: "metric1,'5s','max'",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{1, 2, 3, 4.5, 5},
			"summarize(metric1,'5s','max')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
				},
				namedArgs: map[string]*expr{
					"func": &expr{valStr: "min", etype: etString},
				},
				argString: "metric1,'5s','min'",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{0, 1, 1.5, 2, 5},
			"summarize(metric1,'5s','min')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
				},
				namedArgs: map[string]*expr{
					"func": &expr{valStr: "last", etype: etString},
				},
				argString: "metric1,'5s','last'",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{1, 2, 3, 4.5, 5},
			"summarize(metric1,'5s','last')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
					&expr{valStr: "p50", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{0.5, 1.5, 2, 3, 5},
			"summarize(metric1,'5s','p50')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
					&expr{valStr: "p25", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{0, 1, 2, 3, 5},
			"summarize(metric1,'5s','p25')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
					&expr{valStr: "p99.9", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{1, 2, 3, 4.498, 5},
			"summarize(metric1,'5s','p99.9')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "5s", etype: etString},
					&expr{valStr: "p100.1", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()},
			"summarize(metric1,'5s','p100.1')",
			5,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "1s", etype: etString},
					&expr{valStr: "p50", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5}, 1, now32)},
			},
			[]float64{1, 0, 0, 0.5, 1, 2, 1, 1, 1.5, 2, 3, 2, 2, 1.5, 3, 4, 3, 2, 3, 4.5, 5, 5, 5, 5, 5},
			"summarize(metric1,'1s','p50')",
			1,
			now32,
			now32 + 25*1,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "10min", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2,
					3, 3, 3, 3, 3, 4, 4, 4, 4, 4,
					5, 5, 5, 5, 5}, 60, tenThirtyTwo)},
			},
			[]float64{11, 31, 33},
			"summarize(metric1,'10min')",
			600,
			tenThirty,
			tenThirty + 30*60,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "10min", etype: etString},
					&expr{valStr: "sum", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2,
					3, 3, 3, 3, 3, 4, 4, 4, 4, 4,
					5, 5, 5, 5, 5}, 60, tenThirtyTwo)},
			},
			[]float64{11, 31, 33},
			"summarize(metric1,'10min','sum')",
			600,
			tenThirty,
			tenThirty + 30*60,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "10min", etype: etString},
					&expr{valStr: "sum", etype: etString},
				},
				namedArgs: map[string]*expr{
					"alignToFrom": &expr{target: "true", etype: etName},
				},
				argString: "metric1,'10min','sum',alignToFrom=true",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2,
					3, 3, 3, 3, 3, 4, 4, 4, 4, 4,
					5, 5, 5, 5, 5}, 60, tenThirtyTwo)},
			},
			[]float64{15, 35, 25},
			"summarize(metric1,'10min','sum',alignToFrom=true)",
			600,
			tenThirtyTwo,
			tenThirtyTwo + 25*60,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "10min", etype: etString},
				},
				namedArgs: map[string]*expr{
					"alignToFrom": &expr{target: "true", etype: etName},
					"func":        &expr{valStr: "sum", etype: etString},
				},
				argString: "metric1,'10min',alignToFrom=true,func='sum'",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2,
					3, 3, 3, 3, 3, 4, 4, 4, 4, 4,
					5, 5, 5, 5, 5}, 60, tenThirtyTwo)},
			},
			[]float64{15, 35, 25},
			"summarize(metric1,'10min',alignToFrom=true,func='sum')",
			600,
			tenThirtyTwo,
			tenThirtyTwo + 25*60,
		},
		{
			&expr{
				target: "summarize",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "10min", etype: etString},
				},
				namedArgs: map[string]*expr{
					"alignToFrom": &expr{target: "true", etype: etName},
				},
				argString: "metric1,'10min',alignToFrom=true",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2,
					3, 3, 3, 3, 3, 4, 4, 4, 4, 4,
					5, 5, 5, 5, 5}, 60, tenThirtyTwo)},
			},
			[]float64{15, 35, 25},
			"summarize(metric1,'10min',alignToFrom=true)",
			600,
			tenThirtyTwo,
			tenThirtyTwo + 25*60,
		},
		{
			&expr{
				target: "hitcount",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "30s", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2,
					2, 2, 2, 2, 3, 3,
					3, 3, 3, 4, 4, 4,
					4, 4, 5, 5, 5, 5,
					math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
					5}, 5, now32)},
			},
			[]float64{35, 70, 105, 140, math.NaN(), 25},
			"hitcount(metric1,'30s')",
			30,
			now32,
			now32 + 31*5,
		},
		{
			&expr{
				target: "hitcount",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "1h", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3,
					3, 3, 3, 4, 4, 4, 4, 4, 5, 5, 5, 5,
					5}, 5, tenFiftyNine)},
			},
			[]float64{375},
			"hitcount(metric1,'1h')",
			3600,
			tenFiftyNine,
			tenFiftyNine + 25*5,
		},
		{
			&expr{
				target: "hitcount",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "1h", etype: etString},
					&expr{target: "true", etype: etName},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3,
					3, 3, 3, 4, 4, 4, 4, 4, 5, 5, 5, 5,
					5}, 5, tenFiftyNine)},
			},
			[]float64{105, 270},
			"hitcount(metric1,'1h',true)",
			3600,
			tenFiftyNine - (59 * 60),
			tenFiftyNine + 25*5,
		},
		{
			&expr{
				target: "hitcount",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{valStr: "1h", etype: etString},
				},
				namedArgs: map[string]*expr{
					"alignToInterval": &expr{target: "true", etype: etName},
				},
				argString: "metric1,'1h',alignToInterval=true",
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{makeResponse("metric1", []float64{
					1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3,
					3, 3, 3, 4, 4, 4, 4, 4, 5, 5, 5, 5,
					5}, 5, tenFiftyNine)},
			},
			[]float64{105, 270},
			"hitcount(metric1,'1h',alignToInterval=true)",
			3600,
			tenFiftyNine - (59 * 60),
			tenFiftyNine + 25*5,
		},
	}

	for _, tt := range tests {
		fillArgString(tt.e)
		g, err := evalExpr(tt.e, 0, 1, tt.m)
		if err != nil {
			t.Errorf("failed to eval %v: %s", tt.name, err)
			continue
		}
		if g[0].GetStepTime() != tt.step {
			t.Errorf("bad step for %s:\ngot  %d\nwant %d", g[0].GetName(), g[0].GetStepTime(), tt.step)
		}
		if g[0].GetStartTime() != tt.start {
			t.Errorf("bad start for %s: got %s want %s", g[0].GetName(), time.Unix(int64(g[0].GetStartTime()), 0).Format(time.StampNano), time.Unix(int64(tt.start), 0).Format(time.StampNano))
		}
		if g[0].GetStopTime() != tt.stop {
			t.Errorf("bad stop for %s: got %s want %s", g[0].GetName(), time.Unix(int64(g[0].GetStopTime()), 0).Format(time.StampNano), time.Unix(int64(tt.stop), 0).Format(time.StampNano))
		}

		if !nearlyEqual(g[0].Values, g[0].IsAbsent, tt.w) {
			t.Errorf("failed: %s:\ngot  %+v,\nwant %+v", g[0].GetName(), g[0].Values, tt.w)
		}
		if g[0].GetName() != tt.name {
			t.Errorf("bad name for %s: got %s, want %s", tt.e.GetName(), g[0].GetName(), tt.name)
		}
	}
}

func TestEvalMultipleReturns(t *testing.T) {

	now32 := int32(time.Now().Unix())

	tests := []struct {
		e       *expr
		m       map[metricRequest][]*metricData
		name    string
		results map[string][]*metricData
	}{
		{
			&expr{
				target: "groupByNode",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.*.*"},
					&expr{val: 3, etype: etConst},
					&expr{valStr: "sum", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.*.*", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, 5}, 1, now32),
					makeResponse("metric1.foo.bar1.qux", []float64{6, 7, 8, 9, 10}, 1, now32),
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15}, 1, now32),
					makeResponse("metric1.foo.bar2.qux", []float64{7, 8, 9, 10, 11}, 1, now32),
				},
			},
			"groupByNode",
			map[string][]*metricData{
				"baz": []*metricData{makeResponse("baz", []float64{12, 14, 16, 18, 20}, 1, now32)},
				"qux": []*metricData{makeResponse("qux", []float64{13, 15, 17, 19, 21}, 1, now32)},
			},
		},
		{
			&expr{
				target: "applyByNode",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.*.*"},
					&expr{val: 2, etype: etConst},
					&expr{valStr: "sumSeries(%.baz)", etype: etString},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.*.*", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, 5}, 1, now32),
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15}, 1, now32),
				},
				metricRequest{"metric1.foo.bar1.baz", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, 5}, 1, now32),
				},
				metricRequest{"metric1.foo.bar2.baz", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15}, 1, now32),
				},
			},
			"applyByNode",
			map[string][]*metricData{
				"metric1.foo.bar1": []*metricData{makeResponse("metric1.foo.bar1", []float64{1, 2, 3, 4, 5}, 1, now32)},
				"metric1.foo.bar2": []*metricData{makeResponse("metric1.foo.bar2", []float64{11, 12, 13, 14, 15}, 1, now32)},
			},
		},
		{
			&expr{
				target: "sumSeriesWithWildcards",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.*.*"},
					&expr{val: 1, etype: etConst},
					&expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.*.*", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, 5}, 1, now32),
					makeResponse("metric1.foo.bar1.qux", []float64{6, 7, 8, 9, 10}, 1, now32),
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15}, 1, now32),
					makeResponse("metric1.foo.bar2.qux", []float64{7, 8, 9, 10, 11}, 1, now32),
				},
			},
			"sumSeriesWithWildcards",
			map[string][]*metricData{
				"sumSeriesWithWildcards(metric1.baz)": []*metricData{makeResponse("sumSeriesWithWildcards(metric1.baz)", []float64{12, 14, 16, 18, 20}, 1, now32)},
				"sumSeriesWithWildcards(metric1.qux)": []*metricData{makeResponse("sumSeriesWithWildcards(metric1.qux)", []float64{13, 15, 17, 19, 21}, 1, now32)},
			},
		},
		{
			&expr{
				target: "averageSeriesWithWildcards",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1.foo.*.*"},
					&expr{val: 1, etype: etConst},
					&expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1.foo.*.*", 0, 1}: []*metricData{
					makeResponse("metric1.foo.bar1.baz", []float64{1, 2, 3, 4, 5}, 1, now32),
					makeResponse("metric1.foo.bar1.qux", []float64{6, 7, 8, 9, 10}, 1, now32),
					makeResponse("metric1.foo.bar2.baz", []float64{11, 12, 13, 14, 15}, 1, now32),
					makeResponse("metric1.foo.bar2.qux", []float64{7, 8, 9, 10, 11}, 1, now32),
				},
			},
			"averageSeriesWithWildcards",
			map[string][]*metricData{
				"averageSeriesWithWildcards(metric1.baz)": []*metricData{makeResponse("averageSeriesWithWildcards(metric1.baz)", []float64{6, 7, 8, 9, 10}, 1, now32)},
				"averageSeriesWithWildcards(metric1.qux)": []*metricData{makeResponse("averageSeriesWithWildcards(metric1.qux)", []float64{6.5, 7.5, 8.5, 9.5, 10.5}, 1, now32)},
			},
		},
		{
			&expr{
				target: "highestCurrent",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32),
					makeResponse("metricB", []float64{1, 1, 3, 3, 4, 1}, 1, now32),
					makeResponse("metricC", []float64{1, 1, 3, 3, 4, 15}, 1, now32),
				},
			},
			"highestCurrent",
			map[string][]*metricData{
				"metricA": []*metricData{makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32)},
				"metricC": []*metricData{makeResponse("metricC", []float64{1, 1, 3, 3, 4, 15}, 1, now32)},
			},
		},
		{
			&expr{
				target: "lowestCurrent",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 3, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricB", []float64{1, 1, 3, 3, 4, 1}, 1, now32),
					makeResponse("metricC", []float64{1, 1, 3, 3, 4, 15}, 1, now32),
					makeResponse("metricD", []float64{1, 1, 3, 3, 4, 3}, 1, now32),
					makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32),
				},
			},
			"lowestCurrent",
			map[string][]*metricData{
				"metricA": []*metricData{makeResponse("metricA", []float64{1, 1, 3, 3, 4, 12}, 1, now32)},
				"metricB": []*metricData{makeResponse("metricB", []float64{1, 1, 3, 3, 4, 1}, 1, now32)},
				"metricD": []*metricData{makeResponse("metricD", []float64{1, 1, 3, 3, 4, 3}, 1, now32)},
			},
		},
		{
			&expr{
				target: "limit",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 1, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{0, 0, 1, 0, 0, 0}, 1, now32),
					makeResponse("metricC", []float64{0, 0, 0, 1, 0, 0}, 1, now32),
					makeResponse("metricD", []float64{0, 0, 0, 0, 1, 0}, 1, now32),
					makeResponse("metricE", []float64{0, 0, 0, 0, 0, 1}, 1, now32),
				},
			},
			"limit",
			map[string][]*metricData{
				"metricA": []*metricData{makeResponse("metricA", []float64{0, 1, 0, 0, 0, 0}, 1, now32)},
				"metricB": []*metricData{makeResponse("metricB", []float64{0, 0, 1, 0, 0, 0}, 1, now32)},
			},
		},
		{
			&expr{
				target: "limit",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric1"},
					&expr{val: 20, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric1", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 1, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{0, 0, 1, 0, 0, 0}, 1, now32),
					makeResponse("metricC", []float64{0, 0, 0, 1, 0, 0}, 1, now32),
					makeResponse("metricD", []float64{0, 0, 0, 0, 1, 0}, 1, now32),
					makeResponse("metricE", []float64{0, 0, 0, 0, 0, 1}, 1, now32),
				},
			},
			"limit",
			map[string][]*metricData{
				"metricA": []*metricData{makeResponse("metricA", []float64{0, 1, 0, 0, 0, 0}, 1, now32)},
				"metricB": []*metricData{makeResponse("metricB", []float64{0, 0, 1, 0, 0, 0}, 1, now32)},
				"metricC": []*metricData{makeResponse("metricC", []float64{0, 0, 0, 1, 0, 0}, 1, now32)},
				"metricD": []*metricData{makeResponse("metricD", []float64{0, 0, 0, 0, 1, 0}, 1, now32)},
				"metricE": []*metricData{makeResponse("metricE", []float64{0, 0, 0, 0, 0, 1}, 1, now32)},
			},
		},
		{
			&expr{
				target: "mostDeviant",
				etype:  etFunc,
				args: []*expr{
					&expr{val: 2, etype: etConst},
					&expr{target: "metric*"},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricD", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricE", []float64{4, 7, 7, 7, 7, 1}, 1, now32),
				},
			},
			"mostDeviant",
			map[string][]*metricData{
				"metricB": []*metricData{makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32)},
				"metricE": []*metricData{makeResponse("metricE", []float64{4, 7, 7, 7, 7, 1}, 1, now32)},
			},
		},
		{
			&expr{
				target: "mostDeviant",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
					&expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricD", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricE", []float64{4, 7, 7, 7, 7, 1}, 1, now32),
				},
			},
			"mostDeviant",
			map[string][]*metricData{
				"metricB": []*metricData{makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32)},
				"metricE": []*metricData{makeResponse("metricE", []float64{4, 7, 7, 7, 7, 1}, 1, now32)},
			},
		},
		{
			&expr{
				target: "pearsonClosest",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metricC"},
					&expr{target: "metric*"},
					&expr{val: 2, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricD", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricE", []float64{4, 7, 7, 7, 7, 1}, 1, now32),
				},
				metricRequest{"metricC", 0, 1}: []*metricData{
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			"pearsonClosest",
			map[string][]*metricData{
				"metricC": []*metricData{makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32)},
				"metricD": []*metricData{makeResponse("metricD", []float64{4, 4, 5, 5, 6, 6}, 1, now32)},
			},
		},
		{
			&expr{
				target: "pearsonClosest",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metricC"},
					&expr{target: "metric*"},
					&expr{val: 3, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{0, 0, 0, 0, 0, 0}, 1, now32),
					makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32),
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricD", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
					makeResponse("metricE", []float64{4, 7, 7, 7, 7, 1}, 1, now32),
				},
				metricRequest{"metricC", 0, 1}: []*metricData{
					makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32),
				},
			},
			"pearsonClosest",
			map[string][]*metricData{
				"metricB": []*metricData{makeResponse("metricB", []float64{3, 4, 5, 6, 7, 8}, 1, now32)},
				"metricC": []*metricData{makeResponse("metricC", []float64{4, 4, 5, 5, 6, 6}, 1, now32)},
				"metricD": []*metricData{makeResponse("metricD", []float64{4, 4, 5, 5, 6, 6}, 1, now32)},
			},
		},
		{
			&expr{
				target: "tukeyAbove",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
					&expr{val: 1.5, etype: etConst},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{21, 17, 20, 20, 10, 29}, 1, now32),
					makeResponse("metricB", []float64{20, 18, 21, 19, 20, 20}, 1, now32),
					makeResponse("metricC", []float64{19, 19, 21, 17, 23, 20}, 1, now32),
					makeResponse("metricD", []float64{18, 20, 22, 14, 26, 20}, 1, now32),
					makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32),
				},
			},

			"tukeyAbove",
			map[string][]*metricData{
				"metricA": []*metricData{makeResponse("metricA", []float64{21, 17, 20, 20, 10, 29}, 1, now32)},
				"metricD": []*metricData{makeResponse("metricD", []float64{18, 20, 22, 14, 26, 20}, 1, now32)},
				"metricE": []*metricData{makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32)},
			},
		},
		{
			&expr{
				target: "tukeyAbove",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
					&expr{val: 3, etype: etConst},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{21, 17, 20, 20, 10, 29}, 1, now32),
					makeResponse("metricB", []float64{20, 18, 21, 19, 20, 20}, 1, now32),
					makeResponse("metricC", []float64{19, 19, 21, 17, 23, 20}, 1, now32),
					makeResponse("metricD", []float64{18, 20, 22, 14, 26, 20}, 1, now32),
					makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32),
				},
			},

			"tukeyAbove",
			map[string][]*metricData{
				"metricE": []*metricData{makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32)},
			},
		},
		{
			&expr{
				target: "tukeyBelow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
					&expr{val: 1.5, etype: etConst},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{21, 17, 20, 20, 10, 29}, 1, now32),
					makeResponse("metricB", []float64{20, 18, 21, 19, 20, 20}, 1, now32),
					makeResponse("metricC", []float64{19, 19, 21, 17, 23, 20}, 1, now32),
					makeResponse("metricD", []float64{18, 20, 22, 14, 26, 20}, 1, now32),
					makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32),
				},
			},

			"tukeyBelow",
			map[string][]*metricData{
				"metricA": []*metricData{makeResponse("metricA", []float64{21, 17, 20, 20, 10, 29}, 1, now32)},
				"metricE": []*metricData{makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32)},
			},
		},
		{
			&expr{
				target: "tukeyBelow",
				etype:  etFunc,
				args: []*expr{
					&expr{target: "metric*"},
					&expr{val: 3, etype: etConst},
					&expr{val: 5, etype: etConst},
				},
			},
			map[metricRequest][]*metricData{
				metricRequest{"metric*", 0, 1}: []*metricData{
					makeResponse("metricA", []float64{21, 17, 20, 20, 10, 29}, 1, now32),
					makeResponse("metricB", []float64{20, 18, 21, 19, 20, 20}, 1, now32),
					makeResponse("metricC", []float64{19, 19, 21, 17, 23, 20}, 1, now32),
					makeResponse("metricD", []float64{18, 20, 22, 14, 26, 20}, 1, now32),
					makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32),
				},
			},

			"tukeyBelow",
			map[string][]*metricData{
				"metricE": []*metricData{makeResponse("metricE", []float64{17, 21, 8, 30, 18, 28}, 1, now32)},
			},
		},
	}

	for _, tt := range tests {
		g, err := evalExpr(tt.e, 0, 1, tt.m)
		if err != nil {
			t.Errorf("failed to eval %v: %s", tt.name, err)
			continue
		}
		if g[0] == nil {
			t.Errorf("returned no value %v", tt.name)
			continue
		}
		if g[0].GetStepTime() == 0 {
			t.Errorf("missing step for %+v", g)
		}
		if len(g) != len(tt.results) {
			t.Errorf("unexpected results len: got %d, want %d", len(g), len(tt.results))
		}
		for _, gg := range g {
			r, ok := tt.results[gg.GetName()]
			if !ok {
				t.Errorf("missing result name: %v", gg.GetName())
				continue
			}
			if r[0].GetName() != gg.GetName() {
				t.Errorf("result name mismatch, got\n%#v,\nwant\n%#v", gg.GetName(), r[0].GetName())
			}
			if !reflect.DeepEqual(r[0].Values, gg.Values) || !reflect.DeepEqual(r[0].IsAbsent, gg.IsAbsent) ||
				r[0].GetStartTime() != gg.GetStartTime() ||
				r[0].GetStopTime() != gg.GetStopTime() ||
				r[0].GetStepTime() != gg.GetStepTime() {
				t.Errorf("result mismatch, got\n%#v,\nwant\n%#v", gg, r)
			}
		}
	}
}

func TestExtractMetric(t *testing.T) {

	var tests = []struct {
		input  string
		metric string
	}{
		{
			"f",
			"f",
		},
		{
			"func(f)",
			"f",
		},
		{
			"foo.bar.baz",
			"foo.bar.baz",
		},
		{
			"nonNegativeDerivative(foo.bar.baz)",
			"foo.bar.baz",
		},
		{
			"movingAverage(foo.bar.baz,10)",
			"foo.bar.baz",
		},
		{
			"scale(scaleToSeconds(nonNegativeDerivative(foo.bar.baz),60),60)",
			"foo.bar.baz",
		},
		{
			"divideSeries(foo.bar.baz,baz.qux.zot)",
			"foo.bar.baz",
		},
		{
			"{something}",
			"{something}",
		},
	}

	for _, tt := range tests {
		if m := extractMetric(tt.input); m != tt.metric {
			t.Errorf("extractMetric(%q)=%q, want %q", tt.input, m, tt.metric)
		}
	}
}

func TestEvalCustomFromUntil(t *testing.T) {

	tests := []struct {
		e     *expr
		m     map[metricRequest][]*metricData
		w     []float64
		name  string
		from  int32
		until int32
	}{
		{
			&expr{
				target: "timeFunction",
				etype:  etFunc,
				args: []*expr{
					&expr{valStr: "footime", etype: etString},
				},
			},
			map[metricRequest][]*metricData{},
			[]float64{4200.0, 4260.0, 4320.0},
			"footime",
			4200,
			4350,
		},
	}

	for _, tt := range tests {
		oldValues := map[metricRequest][]*metricData{}
		for key, metrics := range tt.m {
			entry := []*metricData{}
			for _, value := range metrics {
				newValue := metricData{
					FetchResponse: pb.FetchResponse{
						Name:      value.Name,
						StartTime: value.StartTime,
						StopTime:  value.StopTime,
						StepTime:  value.StepTime,
						Values:    make([]float64, len(value.Values)),
						IsAbsent:  make([]bool, len(value.IsAbsent)),
					},
				}

				copy(newValue.Values, value.Values)
				copy(newValue.IsAbsent, value.IsAbsent)
				entry = append(entry, &newValue)
			}

			oldValues[key] = entry
		}

		g, err := evalExpr(tt.e, tt.from, tt.until, tt.m)
		if err != nil {
			t.Errorf("failed to eval %v: %s", tt.name, err)
			continue
		}
		if g[0] == nil {
			t.Errorf("returned no value %v", tt.e.GetName())
			continue
		}

		for key, metrics := range tt.m {
			for i, newValue := range metrics {
				oldValue := oldValues[key][i]
				if !reflect.DeepEqual(oldValue, newValue) {
					t.Errorf("%s: source data was modified key %v index %v want:\n%v\n got:\n%v", tt.e.target, key, i, oldValue, newValue)
				}
			}
		}

		if g[0].GetStepTime() == 0 {
			t.Errorf("missing step for %+v", g)
		}
		if !nearlyEqual(g[0].Values, g[0].IsAbsent, tt.w) {
			t.Errorf("failed: %s: got %+v, want %+v", g[0].GetName(), g[0].Values, tt.w)
		}
		if g[0].GetName() != tt.name {
			t.Errorf("bad name for %+v: got %v, want %v", g, g[0].GetName(), tt.name)
		}
	}
}

const eps = 0.0000000001

func nearlyEqual(a []float64, absent []bool, b []float64) bool {

	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		// "same"
		if absent[i] && math.IsNaN(b[i]) {
			continue
		}
		if absent[i] || math.IsNaN(b[i]) {
			// unexpected NaN
			return false
		}
		// "close enough"
		if math.Abs(v-b[i]) > eps {
			return false
		}
	}

	return true
}

func nearlyEqualMetrics(a, b *metricData) bool {

	if len(a.IsAbsent) != len(b.IsAbsent) {
		return false
	}

	for i := range a.IsAbsent {
		if a.IsAbsent[i] != b.IsAbsent[i] {
			return false
		}
		// "close enough"
		if math.Abs(a.Values[i]-b.Values[i]) > eps {
			return false
		}
	}

	return true
}
