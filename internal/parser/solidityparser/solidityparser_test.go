package solidityparser

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func dumpAST(node *tree_sitter.Node, src []byte, indent int) {
	prefix := strings.Repeat("  ", indent)
	text := node.Utf8Text(src)
	if len(text) > 80 {
		text = text[:80] + "..."
	}
	text = strings.ReplaceAll(text, "\n", "\\n")
	fmt.Printf("%s%s [%d:%d] %q\n", prefix, node.Kind(), node.StartPosition().Row, node.StartPosition().Column, text)
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			dumpAST(child, src, indent+1)
		}
	}
}

func TestDumpAST(t *testing.T) {
	if os.Getenv("DUMP_AST") == "" {
		t.Skip("set DUMP_AST=1 to run")
	}

	src, err := os.ReadFile("testdata/sample.sol")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	lang := tree_sitter.NewLanguage(Language())
	_ = parser.SetLanguage(lang)
	tree := parser.Parse(src, nil)
	defer tree.Close()

	root := tree.RootNode()
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child != nil {
			dumpAST(child, src, 0)
			fmt.Println("---")
		}
	}
}

func TestExtractExportedFuncs(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.sol")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	sigs, err := ExtractExportedFuncs("testdata/sample.sol", src)
	if err != nil {
		t.Fatalf("ExtractExportedFuncs: %v", err)
	}

	var lines []string
	for _, s := range sigs {
		lines = append(lines, s.Text)
	}
	joined := strings.Join(lines, "\n")

	t.Logf("Extracted signatures:\n%s", joined)

	mustContain := []string{
		// Top-level constant
		"uint256 constant MAX_SUPPLY",

		// Top-level struct and enum
		"struct GlobalConfig { ... }",
		"enum Status { Pending, Active, Closed }",

		// Top-level error and event
		"error Unauthorized(address caller)",
		"event GlobalPaused(address indexed by)",

		// Top-level function
		"function add(uint256 a, uint256 b) pure returns (uint256)",

		// User-defined type
		"type Price is uint256",

		// Interface
		"interface IERC721 { ... }",
		"IERC721.function balanceOf(address owner) external view returns (uint256)",
		"IERC721.function ownerOf(uint256 tokenId) external view returns (address)",
		"IERC721.event Transfer(address indexed from, address indexed to, uint256 indexed tokenId)",

		// Library
		"library SafeMath { ... }",
		"SafeMath.function tryAdd(uint256 a, uint256 b) internal pure returns (bool, uint256)",
		"SafeMath.function trySub(uint256 a, uint256 b) internal pure returns (bool, uint256)",

		// Abstract contract
		"contract Ownable { ... }",
		"Ownable.address public owner",
		"Ownable.event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)",
		"Ownable.error OwnableUnauthorizedAccount(address account)",
		"Ownable.modifier onlyOwner()",
		"Ownable.constructor()",
		"Ownable.function transferOwnership(address newOwner) public virtual onlyOwner",
		"Ownable.function renounceOwnership() public virtual onlyOwner",
		"Ownable.function _checkOwner() internal view",

		// Contract with inheritance
		"contract MyToken is Ownable { ... }",
		"MyToken.string public name",
		"MyToken.string public symbol",
		"MyToken.uint8 public decimals",
		"MyToken.uint256 public totalSupply",
		"MyToken.uint256 internal _reserveBalance",

		// MyToken events
		"MyToken.event Transfer(address indexed from, address indexed to, uint256 value)",
		"MyToken.event Approval(address indexed owner, address indexed spender, uint256 value)",

		// MyToken errors
		"MyToken.error InsufficientBalance(uint256 available, uint256 required)",
		"MyToken.error InvalidRecipient(address recipient)",

		// MyToken struct and enum
		"MyToken.struct Checkpoint { ... }",
		"MyToken.enum TokenState { Active, Paused, Deprecated }",

		// MyToken modifier
		"MyToken.modifier whenNotPaused()",

		// MyToken constructor
		"MyToken.constructor(string memory _name, string memory _symbol, uint256 _initialSupply)",

		// MyToken functions
		"MyToken.function transfer(address to, uint256 amount) public whenNotPaused returns (bool)",
		"MyToken.function approve(address spender, uint256 amount) public returns (bool)",
		"MyToken.function transferFrom(address from, address to, uint256 amount) external returns (bool)",
		"MyToken.function mint(address to, uint256 amount) external onlyOwner",
		"MyToken.function burn(uint256 amount) public",
		"MyToken.function transferOwnership(address newOwner) public override onlyOwner",

		// Receive and fallback
		"MyToken.receive() external payable",
		"MyToken.fallback() external payable",

		// GovernanceToken
		"contract GovernanceToken is MyToken, IERC721 { ... }",
		`GovernanceToken.constructor() MyToken("GOV", "GOV", 1000000)`,
		"GovernanceToken.function balanceOf(address owner) external view override returns (uint256)",
		"GovernanceToken.function ownerOf(uint256 tokenId) external view override returns (address)",
		"GovernanceToken.function delegate(address delegatee) public",
		"GovernanceToken.function getVotes(address account) external view returns (uint256)",

		// Mapping state variables
		"MyToken.mapping(address => uint256) public balanceOf",
		"MyToken.mapping(address => mapping(address => uint256)) public allowance",
	}

	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected signature: %q", want)
		}
	}

	mustNotContain := []string{
		"_internalSecret",
		"_beforeTransfer",
		"_secretNonce",
		"pragma",
		"import",
	}

	for _, bad := range mustNotContain {
		if strings.Contains(joined, bad) {
			t.Errorf("should not contain %q, but found it in output", bad)
		}
	}
}

func TestExtractEmptyFile(t *testing.T) {
	sigs, err := ExtractExportedFuncs("empty.sol", []byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sigs) != 0 {
		t.Errorf("expected 0 sigs for empty file, got %d", len(sigs))
	}
}

func TestExtractPrivateOnly(t *testing.T) {
	src := []byte(`
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Secret {
    uint256 private _hidden;

    function _doSecret() private pure returns (uint256) {
        return 42;
    }
}
`)
	sigs, err := ExtractExportedFuncs("private.sol", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var lines []string
	for _, s := range sigs {
		lines = append(lines, s.Text)
	}
	joined := strings.Join(lines, "\n")

	if strings.Contains(joined, "_hidden") {
		t.Errorf("should not contain private state variable _hidden")
	}
	if strings.Contains(joined, "_doSecret") {
		t.Errorf("should not contain private function _doSecret")
	}
	// The contract declaration itself should still appear
	if !strings.Contains(joined, "contract Secret { ... }") {
		t.Errorf("should contain the contract declaration")
	}
}
