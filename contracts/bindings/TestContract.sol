// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract TestContract {
    string public name;
    uint256 public value;
    address public owner;
    
    event ValueUpdated(uint256 indexed newValue, address indexed updater);
    
    constructor(string memory _name) {
        name = _name;
        owner = msg.sender;
        value = 0;
    }
    
    function setValue(uint256 _newValue) external {
        value = _newValue;
        emit ValueUpdated(_newValue, msg.sender);
    }
    
    function getValue() external view returns (uint256) {
        return value;
    }
    
    function getOwner() external view returns (address) {
        return owner;
    }
    
    function getName() external view returns (string memory) {
        return name;
    }
}