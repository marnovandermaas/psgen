package main

import (
	"fmt"
)

type LocalScope struct {
	states     map[string]string
	conditions []string
}

type VerbatimOrState struct {
	str      string
	verbatim bool
}

func (vos *VerbatimOrState) getString(scope *Scope) string {
	if vos.verbatim {
		return vos.str
	} else {
		return scope.getState(vos.str)
	}
}

type ProofCommand interface {
	GenProperty
}

type InStateSubProofCommand struct {
	state string
	seq   SequencedProofSteps
}

type HaveProofCommand struct {
	condition string
	helper    ProofHelper
}

type UseProofCommand struct {
	name   string
	helper ProofHelper
}

type ProofHelper interface {
	HelpProperty
}

type NullProofHelpher struct{}

type SplitProofCase struct {
	condition string
	helper    ProofHelper
}

type SplitProofHelper struct {
	cases []SplitProofCase
}

type GraphInductionNodeDefinition struct {
	name      string
	invariant string
	condition VerbatimOrState
	nextNodes []string
	helper    ProofHelper
}

type GraphInductionProofHelper struct {
	backward       bool
	invariants     map[string]string
	entryCondition string
	entryNodes     []string
	nodes          []GraphInductionNodeDefinition
	scope          LocalScope
}

func (helper *GraphInductionProofHelper) findNode(name string) *GraphInductionNodeDefinition {
	for _, node := range helper.nodes {
		if node.name == name {
			return &node
		}
	}
	panic(fmt.Errorf("no node with name %s", name))
}

type GraphInductionProofCommand struct {
	proof GraphInductionProofHelper
}

type SequencedProofSteps struct {
	scope    LocalScope
	sequence [][]ProofCommand
}

type Lemma struct {
	name string
	seq  SequencedProofSteps
}

type ProofDocument struct {
	defs   map[string][]string
	lemmas []Lemma
}

func blocksToProofHelper(blocks []Block) ProofHelper {
	if len(blocks) == 0 {
		return &NullProofHelpher{}
	}

	if len(blocks) > 1 {
		panic("multi step proof helper")
	}

	switch blocks[0].first.operator {
	case "split":
		blocks[0].first.fixArgs(0)

		cases := make([]SplitProofCase, len(blocks[0].body))
		for b, block := range blocks[0].body {
			if block.first.operator != "case" {
				panic("non case command in split")
			}
			block.first.fixArgs(1)
			cases[b] = SplitProofCase{
				condition: block.first.verbatimArg(0),
				helper:    blocksToProofHelper(block.body),
			}
		}

		return &SplitProofHelper{
			cases: cases,
		}
	case "graph_induction":
		blocks[0].first.fixArgs(0)
		x := blocksToGraphInduction(blocks[0])
		return &x
	default:
		panic("unknown proof helper " + blocks[0].first.operator)
	}
}

func blocksToGraphInduction(root Block) GraphInductionProofHelper {
	cmd := GraphInductionProofHelper{
		backward:       root.first.hasFlag("rev"),
		invariants:     make(map[string]string, 0),
		entryCondition: "",
		entryNodes:     make([]string, 0),
		nodes:          make([]GraphInductionNodeDefinition, 0),
		scope: LocalScope{
			states:     make(map[string]string, 0),
			conditions: make([]string, 0),
		},
	}
	for _, block := range root.body {
		switch block.first.operator {
		case "inv":
			block.first.fixArgs(2)
			cmd.invariants[block.first.wordArg(0)] = block.first.verbatimArg(1)
		case "entry":
			block.first.fixArgs(1)
			cmd.entryCondition = block.first.verbatimArg(0)
			cmd.entryNodes = append(cmd.entryNodes, block.first.nowWordArray()...)
		case "node":
			block.first.fixArgs(3)
			node := GraphInductionNodeDefinition{
				name:      block.first.wordArg(0),
				invariant: block.first.wordArg(1),
				condition: block.first.verbatimOrStateArg(2),
				nextNodes: block.first.stepWordArray(),
				helper:    blocksToProofHelper(block.body),
			}
			cmd.nodes = append(cmd.nodes, node)
		case "cond":
			block.first.fixArgs(1)
			cmd.scope.conditions = append(cmd.scope.conditions, block.first.verbatimArg(0))
		}
	}
	return cmd
}

func blocksToSequenceProof(blocks []Block) SequencedProofSteps {
	seq := SequencedProofSteps{
		scope: LocalScope{
			states:     make(map[string]string, 0),
			conditions: make([]string, 0),
		},
		sequence: make([][]ProofCommand, 1),
	}
	seq.sequence[0] = make([]ProofCommand, 0)

	for _, block := range blocks {
		if block.first.operator == "/" {
			if len(seq.sequence[len(seq.sequence)-1]) != 0 {
				seq.sequence = append(seq.sequence, make([]ProofCommand, 0))
			}
			continue
		}

		cmd := blockToProofCommand(block, &seq.scope)
		if cmd != nil {
			seq.sequence[len(seq.sequence)-1] = append(seq.sequence[len(seq.sequence)-1], cmd)
		}
	}
	return seq
}

func blockToProofCommand(block Block, scope *LocalScope) ProofCommand {
	switch block.first.operator {
	case "in":
		block.first.fixArgs(1)
		return &InStateSubProofCommand{
			state: block.first.wordArg(0),
			seq:   blocksToSequenceProof(block.body),
		}
	case "have":
		block.first.fixArgs(1)
		return &HaveProofCommand{
			condition: block.first.verbatimArg(0),
			helper:    blocksToProofHelper(block.body),
		}
	case "cond":
		block.first.fixArgs(1)
		scope.conditions = append(scope.conditions, block.first.verbatimArg(0))
		return nil
	case "state":
		block.first.fixArgs(2)
		scope.states[block.first.wordArg(0)] = block.first.verbatimArg(1)
		return nil
	case "use":
		block.first.fixArgs(1)
		return &UseProofCommand{
			name:   block.first.wordArg(0),
			helper: blocksToProofHelper(block.body),
		}
	case "graph_induction":
		block.first.fixArgs(0)
		return &GraphInductionProofCommand{
			proof: blocksToGraphInduction(block),
		}
	default:
		panic("unknown operator " + block.first.operator)
	}
}

func blocksToProofDocument(blocks []Block) ProofDocument {
	lemmas := make([]Lemma, 0)
	defs := make(map[string][]string, 0)

	for _, block := range blocks {
		switch block.first.operator {
		case "lemma":
			block.first.fixArgs(1)
			lemmas = append(lemmas, Lemma{
				name: block.first.wordArg(0),
				seq:  blocksToSequenceProof(block.body),
			})
		case "def":
			block.first.fixArgs(1)

			props := make([]string, 0)
			for _, block := range block.body {
				if block.first.operator != "." {
					panic(fmt.Errorf("expected . in def"))
				}
				block.first.fixArgs(1)
				props = append(props, block.first.verbatimArg(0))
			}

			defs[block.first.wordArg(0)] = props
		default:
			panic(fmt.Errorf("bad first operator: %s", block.first.operator))
		}
	}

	return ProofDocument{lemmas: lemmas, defs: defs}
}
