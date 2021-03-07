package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

const TH = 6

var ExecutePipeline = func(jobs ...job) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	in := make(chan interface{})

	for _, job := range jobs {
		wg.Add(1)
		out := make(chan interface{})

		go jobWorker(job, in, out, wg)

		in = out
	}
}

func jobWorker(job job, in, out chan interface{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(out)

	job(in, out)
}

var SingleHash = func(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	mu := &sync.Mutex{}

	for i := range in {
		wg.Add(1)
		go singleHashWorker(i, out, wg, mu)
	}
}

func singleHashWorker(in interface{}, out chan interface{}, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	data := strconv.Itoa(in.(int))

	mu.Lock()
	md5 := DataSignerMd5(data)
	mu.Unlock()

	dataChan := make(chan string)
	go crc32Parallel(data, dataChan)
	signerCrc32 := DataSignerCrc32(md5)
	crc32Data := <-dataChan

	out <- crc32Data + "~" + signerCrc32
}

func crc32Parallel(data string, out chan string) {
	out <- DataSignerCrc32(data)
}

var MultiHash = func(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	for i := range in {
		wg.Add(1)
		go multiHashWorker(i.(string), out, wg)
	}
}

func multiHashWorker(in string, out chan interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	mu := &sync.Mutex{}
	crc32 := &sync.WaitGroup{}

	strs := make([]string, TH)

	for i := 0; i < TH; i++ {
		crc32.Add(1)
		data := strconv.Itoa(i) + in

		go func(arr []string, data string, idx int, wg *sync.WaitGroup, mu *sync.Mutex) {
			defer wg.Done()

			data = DataSignerCrc32(data)

			mu.Lock()
			arr[idx] = data
			mu.Unlock()
		}(strs, data, i, crc32, mu)
	}

	crc32.Wait()

	res := strings.Join(strs, "")

	out <- res
}

var CombineResults = func(in, out chan interface{}) {
	var array []string

	for i := range in {
		array = append(array, i.(string))
	}

	sort.Strings(array)
	res := strings.Join(array, "_")

	out <- res
}
