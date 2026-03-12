// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

// Top-level constant
uint256 constant MAX_SUPPLY = 1000000 * 10 ** 18;

// Top-level struct
struct GlobalConfig {
    address admin;
    uint256 fee;
}

// Top-level enum
enum Status { Pending, Active, Closed }

// Top-level error
error Unauthorized(address caller);

// Top-level event
event GlobalPaused(address indexed by);

// Top-level function
function add(uint256 a, uint256 b) pure returns (uint256) {
    return a + b;
}

// User-defined value type
type Price is uint256;

// Interface
interface IERC721 {
    function balanceOf(address owner) external view returns (uint256);
    function ownerOf(uint256 tokenId) external view returns (address);
    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
}

// Library
library SafeMath {
    function tryAdd(uint256 a, uint256 b) internal pure returns (bool, uint256) {
        unchecked {
            uint256 c = a + b;
            if (c < a) return (false, 0);
            return (true, c);
        }
    }

    function trySub(uint256 a, uint256 b) internal pure returns (bool, uint256) {
        unchecked {
            if (b > a) return (false, 0);
            return (true, a - b);
        }
    }
}

// Abstract contract
abstract contract Ownable {
    address public owner;

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    error OwnableUnauthorizedAccount(address account);

    modifier onlyOwner() {
        require(msg.sender == owner, "Not owner");
        _;
    }

    constructor() {
        owner = msg.sender;
    }

    function transferOwnership(address newOwner) public virtual onlyOwner {
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    function renounceOwnership() public virtual onlyOwner {
        emit OwnershipTransferred(owner, address(0));
        owner = address(0);
    }

    function _checkOwner() internal view {
        require(msg.sender == owner, "Not owner");
    }

    // Private function -- should NOT appear in output
    function _internalSecret() private pure returns (uint256) {
        return 42;
    }
}

// Contract with inheritance
contract MyToken is Ownable {
    string public name;
    string public symbol;
    uint8 public decimals;
    uint256 public totalSupply;
    uint256 internal _reserveBalance;

    // Private state variable -- should NOT appear
    uint256 private _secretNonce;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    error InsufficientBalance(uint256 available, uint256 required);
    error InvalidRecipient(address recipient);

    struct Checkpoint {
        uint256 fromBlock;
        uint256 votes;
    }

    enum TokenState { Active, Paused, Deprecated }

    modifier whenNotPaused() {
        require(true, "Paused");
        _;
    }

    constructor(string memory _name, string memory _symbol, uint256 _initialSupply) {
        name = _name;
        symbol = _symbol;
        decimals = 18;
        totalSupply = _initialSupply;
        balanceOf[msg.sender] = _initialSupply;
    }

    function transfer(address to, uint256 amount) public whenNotPaused returns (bool) {
        if (balanceOf[msg.sender] < amount) {
            revert InsufficientBalance(balanceOf[msg.sender], amount);
        }
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) public returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        allowance[from][msg.sender] -= amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }

    function mint(address to, uint256 amount) external onlyOwner {
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function burn(uint256 amount) public {
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;
        emit Transfer(msg.sender, address(0), amount);
    }

    function transferOwnership(address newOwner) public override onlyOwner {
        super.transferOwnership(newOwner);
    }

    receive() external payable {}
    fallback() external payable {}

    // Private function -- should NOT appear
    function _beforeTransfer(address from, address to) private pure returns (bool) {
        return from != to;
    }
}

// Contract with multiple inheritance
contract GovernanceToken is MyToken, IERC721 {
    mapping(uint256 => address) private _owners;

    constructor() MyToken("GOV", "GOV", 1000000) {}

    function balanceOf(address owner) external view override returns (uint256) {
        return 0;
    }

    function ownerOf(uint256 tokenId) external view override returns (address) {
        return _owners[tokenId];
    }

    function delegate(address delegatee) public {
        // delegation logic
    }

    function getVotes(address account) external view returns (uint256) {
        return 0;
    }
}
