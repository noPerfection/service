-- +goose Up
CREATE TABLE smartcontract (
    network_id varchar(20) NOT NULL,
	address varchar(255) NOT NULL, 
	abi_id varchar(20) NOT NULL,
    transaction_id varchar(255) NOT NULL,
    transaction_index smallint unsigned,
    deployer varchar(255) NOT NULL,
    block_number bigint unsigned NOT NULL, 
	block_timestamp bigint unsigned NOT NULL, 
    CONSTRAINT smartcontract_id PRIMARY KEY (network_id, address),
    FOREIGN KEY (abi_id) REFERENCES abi(abi_id) 
);