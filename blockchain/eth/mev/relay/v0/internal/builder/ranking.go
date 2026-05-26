package builder

import (
	"sort"
	"mev-relay/internal/pb"
)

// RankBundles sorts bundles in descending order of profit
func RankBundles(bundles []*pb.BundleSubmission) []*pb.BundleSubmission {
	sort.SliceStable(bundles, func(i, j int) bool {
		return bundles[i].ProfitEth > bundles[j].ProfitEth
	})
	return bundles
}
