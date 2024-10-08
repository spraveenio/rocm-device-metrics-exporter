/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func fatal(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func parseMF(reader io.Reader) (map[string]*dto.MetricFamily, error) {
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}
	return mf, nil
}

func main() {
	o := flag.String("o", "output.json", "output filepath")
	outCurr := flag.String("out-curr", "output_curr.txt", "raw data output filepath for watch")
	outLast := flag.String("out-last", "output_last.txt", "raw data output filepath for watch")
	w := flag.Bool("w", false, "watch mode")
	showAll := flag.Bool("a", false, "show all metrics")
	interval := flag.Duration("i", 5*time.Second, "interval to pull")

	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Println("Please supply one input source")
		return
	}

	if *w {
		watch(flag.Args()[0], *outCurr, *outLast, *interval, *showAll)
		return
	}

	err := process(flag.Args()[0], *o)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("save result into %s\n", *outCurr)
}

func clearTerminal() {
	cmd := exec.Command("clear") // Use "cls" for Windows
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func getValue(m *dto.MetricFamily) string {
	switch *m.Type {
	case dto.MetricType_COUNTER:
		return fmt.Sprintf("%.2f", *m.Metric[0].Counter.Value)
	case dto.MetricType_GAUGE:
		return fmt.Sprintf("%.2f", *m.Metric[0].Gauge.Value)
	case dto.MetricType_HISTOGRAM:
		return fmt.Sprintf("%s", m.Metric[0].Histogram.String())
	}
	return ""
}

func watch(input, outCurr, outLast string, interval time.Duration, showAll bool) {
	var last, current map[string]*dto.MetricFamily
	var bufLast, bufCurrent []byte
	if !strings.HasPrefix(input, "http") {
		input = "http://" + input
	}
	for {
		resp, err := http.Get(input)
		if err != nil {
			log.Fatalln(err)
		}

		bufCurrent, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}

		current, err = parseMF(bytes.NewBuffer(bufCurrent))
		if err != nil {
			log.Fatalln(err)
		}

		err = resp.Body.Close()
		if err != nil {
			log.Fatalln(err)
		}

		if last != nil {
			err = os.WriteFile(outLast, bufLast, 0644)
			if err != nil {
				log.Fatalln(err)
			}
			err = os.WriteFile(outCurr, bufCurrent, 0644)
			if err != nil {
				log.Fatalln(err)
			}
			clearTerminal()
			notExistsInCurr := []*dto.MetricFamily{}
			diffs := [][]*dto.MetricFamily{}
			for k, currItem := range current {
				lastItem, ok := last[k]
				if !ok {
					notExistsInCurr = append(notExistsInCurr, currItem)
					continue
				}

				delete(last, k)

				if reflect.DeepEqual(lastItem, currItem) {
					continue
				}
				diffs = append(diffs, []*dto.MetricFamily{lastItem, currItem})
			}
			writer := tabwriter.NewWriter(os.Stdout, 5, 1, 1, ' ', 0)

			writer.Write([]byte("\n"))
			writer.Write([]byte("Metric\tLast Iteration\tCurrent Iteration\n"))

			notExistsInLast := []*dto.MetricFamily{}
			for _, v := range last {
				notExistsInLast = append(notExistsInLast, v)
			}
			sort.Slice(notExistsInLast, func(i, j int) bool {
				return notExistsInLast[i].GetName() < notExistsInLast[j].GetName()
			})
			for _, v := range notExistsInLast {
				writer.Write([]byte(fmt.Sprintf("%s:%s\t%s\n", v.GetName(), v.Type, getValue(v))))
			}

			sort.Slice(notExistsInCurr, func(i, j int) bool {
				return notExistsInCurr[i].GetName() < notExistsInCurr[j].GetName()
			})
			for _, v := range notExistsInCurr {
				writer.Write([]byte(fmt.Sprintf("%s:%s\t\t%s\n", v.GetName(), v.Type, getValue(v))))
			}

			for _, v := range diffs {
				writer.Write([]byte(fmt.Sprintf("%s:%s\t%s\t%s\n", v[0].GetName(), v[0].Type,
					getValue(v[0]), getValue(v[1]))))
			}
			writer.Flush()
		}
		time.Sleep(interval)
		last = current
		bufLast = bufCurrent
	}
}

func writeOutput(filename string, data interface{}) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, content, 0644)
}

func process(input, output string) error {
	var reader io.ReadCloser
	_, err := os.Stat(input)
	if err == nil {
		reader, err = os.Open(input)
		if err != nil {
			return err
		}
	} else {
		if !strings.HasPrefix(input, "http") {
			input = "http://" + input
		}
		resp, err := http.Get(input)
		if err != nil {
			return err
		}
		reader = resp.Body
	}
	defer reader.Close()
	mf, err := parseMF(reader)
	if err != nil {
		return err
	}

	values := make([]*dto.MetricFamily, len(mf))
	index := 0
	for _, v := range mf {
		values[index] = v
		index++
	}
	return writeOutput(output, values)
}
