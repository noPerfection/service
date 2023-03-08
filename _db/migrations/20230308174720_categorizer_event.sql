-- +goose Up
CREATE TABLE categorizer_event (
    network_id varchar(20) NOT NULL,
	address varchar(255) NOT NULL,
    transaction_id varchar(55) NOT NULL,
    transaction_index smallint unsigned,
    block_number bigint UNSIGNED,
    block_timestamp bigint UNSIGNED,
    log_index smallint unsigned,
    event_name varchar(255) NOT NULL,
    event_parameters json,
    CONSTRAINT event_id PRIMARY KEY (network_id, transaction_id, transaction_index, log_index),
    INDEX (block_number, block_timestamp, event_name),
    FOREIGN KEY (network_id, address) REFERENCES categorizer_smartcontract(network_id, address) 
);
