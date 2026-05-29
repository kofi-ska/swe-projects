package graph

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"time"
)

type Digraph map[string][]string

type FlowNetwork map[string]map[string]int

type Routing struct {
	shards []string
}

type Authority struct {
	ShardID    string    `json:"shardId"`
	LeaseID    string    `json:"leaseId"`
	Epoch      uint64    `json:"epoch"`
	FenceToken uint64    `json:"fenceToken"`
	IssuedAt   time.Time `json:"issuedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

func NewRouting(shards []string) Routing {
	out := make([]string, 0, len(shards))
	seen := map[string]struct{}{}
	for _, shard := range shards {
		if shard == "" {
			continue
		}
		if _, ok := seen[shard]; ok {
			continue
		}
		seen[shard] = struct{}{}
		out = append(out, shard)
	}
	sort.Strings(out)
	return Routing{shards: out}
}

func (r Routing) Shards() []string {
	out := make([]string, len(r.shards))
	copy(out, r.shards)
	return out
}

func (r Routing) Route(key string) string {
	if len(r.shards) == 0 {
		return ""
	}
	var (
		bestShard string
		bestScore uint64
	)
	for _, shard := range r.shards {
		sum := sha256.Sum256([]byte(key + ":" + shard))
		score := binary.BigEndian.Uint64(sum[:8])
		if bestShard == "" || score > bestScore {
			bestShard = shard
			bestScore = score
		}
	}
	return bestShard
}

func NewAuthority(shardID, leaseID string, epoch uint64, ttl time.Duration, now time.Time) Authority {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Authority{
		ShardID:    shardID,
		LeaseID:    leaseID,
		Epoch:      epoch,
		FenceToken: epoch,
		IssuedAt:   now,
		ExpiresAt:  now.Add(ttl),
	}
}

func (a Authority) Valid(now time.Time) bool {
	if a.ShardID == "" || a.LeaseID == "" {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return now.Before(a.ExpiresAt)
}

func (a Authority) Renew(ttl time.Duration, now time.Time) Authority {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	a.IssuedAt = now
	a.ExpiresAt = now.Add(ttl)
	return a
}

func (a Authority) NextEpoch(now time.Time, ttl time.Duration) Authority {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	a.Epoch++
	a.FenceToken = a.Epoch
	a.IssuedAt = now
	a.ExpiresAt = now.Add(ttl)
	return a
}

func AuthorityKey(shardID string, epoch uint64, fence uint64) string {
	return fmt.Sprintf("%s:%d:%d", shardID, epoch, fence)
}

func Reachable(g Digraph, start, target string) bool {
	if start == target {
		return true
	}
	seen := map[string]struct{}{start: {}}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g[cur] {
			if next == target {
				return true
			}
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return false
}

func TopologicalSort(g Digraph) ([]string, error) {
	inDegree := map[string]int{}
	nodes := map[string]struct{}{}
	for from, outs := range g {
		nodes[from] = struct{}{}
		for _, to := range outs {
			nodes[to] = struct{}{}
			inDegree[to]++
		}
		if _, ok := inDegree[from]; !ok {
			inDegree[from] = inDegree[from]
		}
	}
	queue := make([]string, 0, len(nodes))
	for n := range nodes {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)
	order := make([]string, 0, len(nodes))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		order = append(order, n)
		outs := append([]string(nil), g[n]...)
		sort.Strings(outs)
		for _, to := range outs {
			inDegree[to]--
			if inDegree[to] == 0 {
				queue = append(queue, to)
				sort.Strings(queue)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("graph contains a cycle")
	}
	return order, nil
}

func SCC(g Digraph) [][]string {
	order := make([]string, 0, len(g))
	seen := map[string]struct{}{}
	var dfs1 func(string)
	dfs1 = func(n string) {
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		for _, nxt := range g[n] {
			dfs1(nxt)
		}
		order = append(order, n)
	}
	nodes := allNodes(g)
	for n := range nodes {
		dfs1(n)
	}
	rev := reverse(g)
	seen = map[string]struct{}{}
	comps := [][]string{}
	var dfs2 func(string, *[]string)
	dfs2 = func(n string, comp *[]string) {
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		*comp = append(*comp, n)
		for _, nxt := range rev[n] {
			dfs2(nxt, comp)
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		n := order[i]
		if _, ok := seen[n]; ok {
			continue
		}
		comp := []string{}
		dfs2(n, &comp)
		sort.Strings(comp)
		comps = append(comps, comp)
	}
	sort.Slice(comps, func(i, j int) bool {
		if len(comps[i]) == 0 || len(comps[j]) == 0 {
			return len(comps[i]) < len(comps[j])
		}
		return comps[i][0] < comps[j][0]
	})
	return comps
}

func MaxFlow(net FlowNetwork, source, sink string) int {
	residual := cloneFlow(net)
	flow := 0
	for {
		parent, ok := bfsResidual(residual, source, sink)
		if !ok {
			return flow
		}
		bottleneck := int(^uint(0) >> 1)
		for v := sink; v != source; v = parent[v] {
			u := parent[v]
			if cap := residual[u][v]; cap < bottleneck {
				bottleneck = cap
			}
		}
		for v := sink; v != source; v = parent[v] {
			u := parent[v]
			residual[u][v] -= bottleneck
			if residual[v] == nil {
				residual[v] = map[string]int{}
			}
			residual[v][u] += bottleneck
		}
		flow += bottleneck
	}
}

func MinCut(net FlowNetwork, source, sink string) map[string]struct{} {
	residual := cloneFlow(net)
	for {
		parent, ok := bfsResidual(residual, source, sink)
		if !ok {
			break
		}
		bottleneck := int(^uint(0) >> 1)
		for v := sink; v != source; v = parent[v] {
			u := parent[v]
			if cap := residual[u][v]; cap < bottleneck {
				bottleneck = cap
			}
		}
		for v := sink; v != source; v = parent[v] {
			u := parent[v]
			residual[u][v] -= bottleneck
			if residual[v] == nil {
				residual[v] = map[string]int{}
			}
			residual[v][u] += bottleneck
		}
	}
	seen := map[string]struct{}{}
	queue := []string{source}
	seen[source] = struct{}{}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for v, cap := range residual[u] {
			if cap <= 0 {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			queue = append(queue, v)
		}
	}
	return seen
}

func allNodes(g Digraph) map[string]struct{} {
	nodes := map[string]struct{}{}
	for from, outs := range g {
		nodes[from] = struct{}{}
		for _, to := range outs {
			nodes[to] = struct{}{}
		}
	}
	return nodes
}

func reverse(g Digraph) Digraph {
	rev := Digraph{}
	for from, outs := range g {
		if rev[from] == nil {
			rev[from] = []string{}
		}
		for _, to := range outs {
			rev[to] = append(rev[to], from)
		}
	}
	return rev
}

func cloneFlow(net FlowNetwork) FlowNetwork {
	out := FlowNetwork{}
	for from, outs := range net {
		out[from] = map[string]int{}
		for to, cap := range outs {
			out[from][to] = cap
		}
	}
	return out
}

func bfsResidual(residual FlowNetwork, source, sink string) (map[string]string, bool) {
	parent := map[string]string{}
	seen := map[string]struct{}{source: {}}
	queue := []string{source}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for v, cap := range residual[u] {
			if cap <= 0 {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			parent[v] = u
			if v == sink {
				return parent, true
			}
			seen[v] = struct{}{}
			queue = append(queue, v)
		}
	}
	return nil, false
}
