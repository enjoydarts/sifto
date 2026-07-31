package inngest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestProcessItemModelSplitSelectionRunsInsideDurableStep(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "process_item_flow.go", nil, 0)
	if err != nil {
		t.Fatalf("parse process_item_flow.go: %v", err)
	}

	const selector = "ChooseSplitPrimaryModelWithUsage"
	totalSelections := countSelectorCalls(file, selector)
	durableSelections := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isStepRunCall(call.Fun) {
			return true
		}
		for _, arg := range call.Args {
			durableSelections += countSelectorCalls(arg, selector)
		}
		return true
	})

	if totalSelections != 2 {
		t.Fatalf("model split selections = %d, want 2", totalSelections)
	}
	if durableSelections != totalSelections {
		t.Fatalf("durable model split selections = %d, want %d; selection outside step.Run is replayed by Inngest", durableSelections, totalSelections)
	}
}

func countSelectorCalls(node ast.Node, selector string) int {
	count := 0
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == selector {
			count++
		}
		return true
	})
	return count
}

func isStepRunCall(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Run" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "step"
}
