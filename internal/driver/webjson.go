package driver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/pprof/internal/graph"
	"github.com/google/pprof/internal/measurement"
	"github.com/google/pprof/internal/report"
)

func (ui *webInterface) topData(w http.ResponseWriter, req *http.Request) {
	rpt, errList := ui.makeReport(w, req, []string{"top"}, func(cfg *config) {
		cfg.NodeCount = 500
	})
	if rpt == nil {
		ui.options.UI.PrintErr(errList)
		http.Error(w, "error genereating report"+strings.Join(errList, ";"), http.StatusInternalServerError)
		return
	}
	top, _ := report.TextItems(rpt)
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(top); err != nil {
		http.Error(w, "error serializing top", http.StatusInternalServerError)
		ui.options.UI.PrintErr(err)
	}
}

func (ui *webInterface) flamegraphData(w http.ResponseWriter, req *http.Request) {
	// Force the call tree so that the graph is a tree.
	// Also do not trim the tree so that the flame graph contains all functions.
	rpt, errList := ui.makeReport(w, req, []string{"svg"}, func(cfg *config) {
		cfg.CallTree = true
		cfg.Trim = false
	})
	if rpt == nil {
		ui.options.UI.PrintErr(errList)
		http.Error(w, "error genereating report"+strings.Join(errList, ";"), http.StatusInternalServerError)
		return
	}

	// Generate dot graph.
	g, config := report.GetDOT(rpt)
	var nodes []*treeNode
	nroots := 0
	rootValue := int64(0)
	nodeArr := []string{}
	nodeMap := map[*graph.Node]*treeNode{}
	// Make all nodes and the map, collect the roots.
	for _, n := range g.Nodes {
		v := n.CumValue()
		fullName := n.Info.PrintableName()
		node := &treeNode{
			Name:      graph.ShortenFunctionName(fullName),
			FullName:  fullName,
			Cum:       v,
			CumFormat: config.FormatValue(v),
			Percent:   strings.TrimSpace(measurement.Percentage(v, config.Total)),
		}
		nodes = append(nodes, node)
		if len(n.In) == 0 {
			nodes[nroots], nodes[len(nodes)-1] = nodes[len(nodes)-1], nodes[nroots]
			nroots++
			rootValue += v
		}
		nodeMap[n] = node
		// Get all node names into an array.
		nodeArr = append(nodeArr, n.Info.Name)
	}
	// Populate the child links.
	for _, n := range g.Nodes {
		node := nodeMap[n]
		for child := range n.Out {
			node.Children = append(node.Children, nodeMap[child])
		}
	}

	rootNode := &treeNode{
		Name:      "root",
		FullName:  "root",
		Cum:       rootValue,
		CumFormat: config.FormatValue(rootValue),
		Percent:   strings.TrimSpace(measurement.Percentage(rootValue, config.Total)),
		Children:  nodes[0:nroots],
	}

	// JSON marshalling flame graph
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(rootNode); err != nil {
		http.Error(w, "error serializing flame graph", http.StatusInternalServerError)
		ui.options.UI.PrintErr(err)
	}
}
