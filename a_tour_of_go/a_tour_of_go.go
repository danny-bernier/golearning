// TODO https://go.dev/doc/effective_go
// TODO https://go.dev/doc/code
// https://go.dev/blog/slices-intro
package atourofgo

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

var Days = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

/*
https://go.dev/blog/slices-intro#a-possible-gotcha

To fix this problem one can copy the interesting data to a new slice before returning it:

var digitRegexp = regexp.MustCompile("[0-9]+")
func CopyDigits(filename string) []byte {
    b, _ := os.ReadFile(filename)
    b = digitRegexp.Find(b)
    c := make([]byte, len(b))
    copy(c, b)
    return c
}
A more concise version of this function could be constructed by using append. This is left as an exercise for the reader.
*/

var digitRegexp = regexp.MustCompile("[0-9]+")

func CopyDigits(filename string) (match []byte) {
	b, _ := os.ReadFile(filename)
	match = append(match, digitRegexp.Find(b)...)
	return
}

/*
https://go.dev/tour/moretypes/18

Exercise: Slices
Implement Pic. It should return a slice of length dy, each element of which is a slice of dx 8-bit unsigned integers. When you run the program, it will display your picture, interpreting the integers as grayscale (well, bluescale) values.

The choice of image is up to you. Interesting functions include (x+y)/2, x*y, and x^y.

(You need to use a loop to allocate each []uint8 inside the [][]uint8.)

(Use uint8(intValue) to convert between types.)
*/

func Pic(dx, dy int) [][]uint8 {
	p := make([][]uint8, dy)
	for y := range p {
		p[y] = make([]uint8, dx)
		for x := range p[y] {
			p[y][x] = uint8(x ^ y)
		}
	}
	return p
}

/*
https://go.dev/tour/moretypes/23

Exercise: Maps
Implement WordCount. It should return a map of the counts of each “word” in the string s. The wc.Test function runs a test suite against the provided function and prints success or failure.

You might find strings.Fields helpful.
*/

func WordCount(s string) map[string]int {
	wordCount := make(map[string]int)
	for _, w := range strings.Split(s, " ") {
		wordCount[w] = wordCount[w] + 1
	}
	return wordCount
}

/*
https://go.dev/tour/moretypes/26
Exercise: Fibonacci closure
Let's have some fun with functions.

Implement a fibonacci function that returns a function (a closure) that returns successive fibonacci numbers (0, 1, 1, 2, 3, 5, ...).
*/

func fibonacci() func() int {
	lastL := 0
	lastR := 0
	return func() int {
		if lastR == 0 {
			lastR = 1
			return 0
		}
		next := lastL + lastR
		lastL = lastR
		lastR = next
		return lastL
	}
}

/*
https://go.dev/tour/methods/18

Exercise: Stringers
Make the IPAddr type implement fmt.Stringer to print the address as a dotted quad.
For instance, IPAddr{1, 2, 3, 4} should print as "1.2.3.4".
*/

type IPAddr [4]byte

func (ip IPAddr) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}

/*
https://go.dev/tour/methods/22
Exercise: Readers
Implement a Reader type that emits an infinite stream of the ASCII character 'A'.
*/

type MyReader struct{}

func (MyReader) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = 'A'
	}
	return len(b), nil
}

/*
https://go.dev/tour/methods/23
Exercise: rot13Reader
A common pattern is an io.Reader that wraps another io.Reader, modifying the stream in some way.
For example, the gzip.NewReader function takes an io.Reader (a stream of compressed data) and returns a *gzip.Reader that also implements io.Reader (a stream of the decompressed data).
Implement a rot13Reader that implements io.Reader and reads from an io.Reader, modifying the stream by applying the rot13 substitution cipher to all alphabetical characters.
The rot13Reader type is provided for you. Make it an io.Reader by implementing its Read method.
*/

var Rot13Lookup = map[byte]byte{
	'A': 'N', 'B': 'O', 'C': 'P', 'D': 'Q', 'E': 'R', 'F': 'S', 'G': 'T',
	'H': 'U', 'I': 'V', 'J': 'W', 'K': 'X', 'L': 'Y', 'M': 'Z', 'N': 'A',
	'O': 'B', 'P': 'C', 'Q': 'D', 'R': 'E', 'S': 'F', 'T': 'G', 'U': 'H',
	'V': 'I', 'W': 'J', 'X': 'K', 'Y': 'L', 'Z': 'M',

	'a': 'n', 'b': 'o', 'c': 'p', 'd': 'q', 'e': 'r', 'f': 's', 'g': 't',
	'h': 'u', 'i': 'v', 'j': 'w', 'k': 'x', 'l': 'y', 'm': 'z', 'n': 'a',
	'o': 'b', 'p': 'c', 'q': 'd', 'r': 'e', 's': 'f', 't': 'g', 'u': 'h',
	'v': 'i', 'w': 'j', 'x': 'k', 'y': 'l', 'z': 'm',
}

type rot13Reader struct {
	r io.Reader
}

func (r13 *rot13Reader) Read(b []byte) (int, error) {
	n, err := r13.r.Read(b)
	for i := 0; i < n; i += 1 {
		if r, ok := Rot13Lookup[b[i]]; ok {
			b[i] = r
		}
	}
	return n, err
}

/*
https://go.dev/tour/generics/2
In addition to generic functions, Go also supports generic types. A type can be parameterized with a type parameter, which could be useful for implementing generic data structures.
This example demonstrates a simple type declaration for a singly-linked list holding any type of value.
As an exercise, add some functionality to this list implementation.
*/
type List[T any] struct {
	next *List[T]
	val  T
}

func (l *List[T]) Len() int {
	count := 1
	curr := l
	for curr.next != nil {
		curr = curr.next
		count += 1
	}
	return count
}

func (l *List[T]) Append(val T) {
	if l == nil {
		panic("cannot append to a nil list pointer")
	}

	curr := l
	for curr.next != nil {
		curr = curr.next
	}
	curr.next = &List[T]{nil, val}
}

func (l *List[T]) SetAt(index int, val T) error {
	if l == nil {
		return fmt.Errorf("list is nil")
	}

	i := 0
	curr := l
	for curr.next != nil && i < index {
		curr = curr.next
		i += 1
	}
	if i < index {
		return fmt.Errorf("index %d out of bounds for List %p with length %d", index, l, i+1)
	}
	curr.val = val
	return nil
}

func (l *List[T]) Get() (val T, err error) {
	if l == nil {
		err = fmt.Errorf("list is nil")
		return
	}

	curr := l
	for curr.next != nil {
		curr = curr.next
	}
	val = curr.val
	return
}

func (l *List[T]) GetAt(index int) (val T, err error) {
	if l == nil {
		err = fmt.Errorf("list is nil")
		return
	}

	i := 0
	curr := l
	for curr.next != nil && i < index {
		curr = curr.next
		i += 1
	}
	if i < index {
		err = fmt.Errorf("index %d out of bounds for List %p with length %d", index, l, i+1)
		return
	}
	val = curr.val
	return
}

/*
https://go.dev/tour/concurrency/8
Exercise: Equivalent Binary Trees
1. Implement the Walk function.
2. Test the Walk function.
The function tree.New(k) constructs a randomly-structured (but always sorted) binary tree holding the values k, 2k, 3k, ..., 10k.
Create a new channel ch and kick off the walker:
go Walk(tree.New(1), ch)
Then read and print 10 values from the channel. It should be the numbers 1, 2, 3, ..., 10.
3. Implement the Same function using Walk to determine whether t1 and t2 store the same values.
4. Test the Same function.
Same(tree.New(1), tree.New(1)) should return true, and Same(tree.New(1), tree.New(2)) should return false.
*/
type Tree struct {
	Left  *Tree
	Value int
	Right *Tree
}

func Walk(t *Tree, ch chan int) {
	if t.Left != nil {
		walk(t.Left, ch)
	}
	ch <- t.Value
	if t.Right != nil {
		walk(t.Right, ch)
	}
	close(ch)
}

func walk(t *Tree, ch chan int) {
	if t.Left != nil {
		walk(t.Left, ch)
	}
	ch <- t.Value
	if t.Right != nil {
		walk(t.Right, ch)
	}
}

func Same(t1, t2 *Tree) bool {
	ch1, ch2 := make(chan int), make(chan int)
	go Walk(t1, ch1)
	go Walk(t2, ch2)
	for {
		v1, ok1 := <-ch1
		v2, ok2 := <-ch2

		if !ok1 && !ok2 {
			return true
		}

		if !ok1 || !ok2 || v1 != v2 {
			return false
		}
	}
}

/*
https://go.dev/tour/concurrency/10
Exercise: Web Crawler
In this exercise you'll use Go's concurrency features to parallelize a web crawler.
Modify the Crawl function to fetch URLs in parallel without fetching the same URL twice.
Hint: you can keep a cache of the URLs that have been fetched on a map, but maps alone are not safe for concurrent use!
*/

type Fetcher interface {
	Fetch(url string) (body string, urls []string, err error)
}

var crawlMap = make(map[string]string)
var crawlMu sync.Mutex

func Crawl(url string, depth int, fetcher Fetcher) {
	if depth <= 0 {
		return
	}
	crawlMu.Lock()
	if _, ok := crawlMap[url]; !ok {
		body, urls, err := fetcher.Fetch(url)
		if err != nil {
			panic(err)
		}
		crawlMap[url] = body
		crawlMu.Unlock()
		for _, url := range urls {
			go Crawl(url, depth-1, fetcher)
		}
	}
}

//TODO
//https://go.dev/tour/concurrency/11
