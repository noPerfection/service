-- +goose Up
CREATE TABLE categorizer_smartcontract (
    network_id varchar(20) NOT NULL,
	address varchar(255) NOT NULL,
    block_number bigint UNSIGNED,
    block_timestamp bigint UNSIGNED,
    CONSTRAINT smartcontract_id PRIMARY KEY (network_id, address)
);
