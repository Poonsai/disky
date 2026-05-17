// Package scan walks a directory tree and produces a tree.Node hierarchy
// suitable for browsing in the disky TUI.
package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Poonsai/disky/internal/tree"
)

// Concurrency model:
// A fixed pool of N = NumCPU workers reads from an unbounded shared
// queue protected by a mutex + cond. Each job is one directory to walk.
// Walking emits new jobs (subdirectories) back onto the queue.
//
// We use a worker pool rather than a goroutine-per-directory because the
// latter creates a goroutine for every subdirectory found, which on a
// large Windows drive (millions of node_modules / .git / WinSxS dirs)
// queues hundreds of thousands of goroutine stacks before backpressure
// kicks in — easily gigabytes of stack memory.
//
// Each tree.Node is written by exactly one worker (the one that walks
// its parent). The post-scan reads in ComputeSizes/Sort run on the
// main goroutine after wg.Wait, providing the happens-before edge.

// Progress is updated atomically during a scan so the UI can render counters.
// Use Progress.CurrentPath() to read the latest path string safely.
type Progress struct {
	Items       int64
	Bytes       int64
	currentPath atomic.Value // string
}

func (p *Progress) CurrentPath() string {
	if p == nil {
		return ""
	}
	v := p.currentPath.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

type job struct {
	path string
	node *tree.Node
}

// queue is an unbounded LIFO of pending directory jobs, protected by a
// mutex and a cond for blocking pop. LIFO gives DFS-like cache locality;
// FIFO would also work but tends to fan out and trash the dentry cache.
type queue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	items   []job
	pending int  // queued + currently-being-processed
	done    bool // set when ctx canceled or pending reaches 0
}

func newQueue() *queue {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(j job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done {
		// Already canceled or drained — don't grow the queue with work
		// that will never be processed.
		return
	}
	q.items = append(q.items, j)
	q.pending++
	q.cond.Signal()
}

// pop blocks until either a job is available or the queue is done. The
// bool return is false when no more work will arrive.
func (q *queue) pop() (job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.done {
		q.cond.Wait()
	}
	if q.done {
		return job{}, false
	}
	j := q.items[len(q.items)-1]
	q.items = q.items[:len(q.items)-1]
	return j, true
}

// finish marks one job as completed. When pending reaches zero (and we
// weren't already canceled) the queue transitions to done so all
// waiting workers can return.
func (q *queue) finish() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending--
	if q.pending == 0 && !q.done {
		q.done = true
		q.cond.Broadcast()
	}
}

// cancel forces the queue done immediately so blocked workers return.
func (q *queue) cancel() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.done {
		q.done = true
		q.cond.Broadcast()
	}
}

func Scan(ctx context.Context, root string, p *Progress) (*tree.Node, error) {
	rootNode := &tree.Node{Name: filepath.Clean(root), IsDir: true}

	q := newQueue()
	q.push(job{rootNode.Name, rootNode})

	// Watch for ctx cancel and propagate it into the queue. cleanupDone
	// stops this watcher when the scan finishes naturally, otherwise
	// it would leak waiting on ctx.Done forever.
	cleanupDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.cancel()
		case <-cleanupDone:
		}
	}()

	seen := newSeenSet()

	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				j, ok := q.pop()
				if !ok {
					return
				}
				walkOne(ctx, j, q, p, seen)
				q.finish()
			}
		}()
	}
	wg.Wait()
	close(cleanupDone)

	tree.ComputeSizes(rootNode)
	tree.Sort(rootNode)

	if err := ctx.Err(); err != nil {
		return rootNode, err
	}
	return rootNode, nil
}

func walkOne(ctx context.Context, j job, q *queue, p *Progress, seen *seenSet) {
	if p != nil {
		p.currentPath.Store(j.path)
	}

	entries, err := os.ReadDir(j.path)
	if err != nil {
		j.node.Err = err
		return
	}
	var local []*tree.Node
	for _, entry := range entries {
		// Check ctx inside the entry loop so cancel takes effect promptly
		// even when walking a directory with millions of entries.
		if ctx.Err() != nil {
			return
		}
		childPath := filepath.Join(j.path, entry.Name())
		info, err := os.Lstat(childPath)
		if err != nil {
			continue
		}
		isLink := info.Mode()&os.ModeSymlink != 0
		// On Windows, directory junctions and mount points are reparse
		// points but Go does NOT report them as symlinks via Mode().
		// Without an explicit reparse-point check, the walker descends
		// into junctions like C:\Documents and Settings → C:\Users,
		// double-counting bytes and risking loops.
		isReparse := isReparsePoint(info)
		child := &tree.Node{
			Name:   entry.Name(),
			Parent: j.node,
			IsDir:  info.IsDir() && !isLink && !isReparse,
		}
		switch {
		case isLink || isReparse:
			// Symlinks and Windows reparse points (junctions, mount
			// points, etc.) are not followed. They count as 0 bytes
			// to avoid double counting and to prevent loops.
			child.Size = 0
		case info.IsDir():
			q.push(job{childPath, child})
		default:
			size := info.Size()
			// NTFS makes heavy use of hard links (WinSxS in particular).
			// Counting each link at full size inflates totals well beyond
			// what Explorer reports. Track the unique file ID and only
			// charge the first occurrence; subsequent links count as 0.
			if size > 0 && !seen.firstSeen(childPath, info) {
				size = 0
			}
			child.Size = size
			if size > 0 && p != nil {
				atomic.AddInt64(&p.Items, 1)
				atomic.AddInt64(&p.Bytes, size)
			} else if size == 0 && p != nil {
				// Even zero-byte and deduped files bump the item count
				// so the UI's "items scanned" still climbs visibly.
				atomic.AddInt64(&p.Items, 1)
			}
		}
		local = append(local, child)
	}
	j.node.Children = local
}

// seenSet tracks unique file IDs across the scan so that hard-linked
// files are only counted once. Falls back to no-op on platforms where
// fileUniqueID isn't implemented.
type seenSet struct {
	m sync.Map
}

func newSeenSet() *seenSet { return &seenSet{} }

// firstSeen returns true if this is the first time we've seen this
// underlying file. Subsequent hard links to the same inode return false.
// If the platform can't produce a stable ID, this always returns true
// (no dedup).
func (s *seenSet) firstSeen(path string, info os.FileInfo) bool {
	id, ok := fileUniqueID(path, info)
	if !ok {
		return true
	}
	_, loaded := s.m.LoadOrStore(id, struct{}{})
	return !loaded
}
