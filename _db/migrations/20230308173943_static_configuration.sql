-- +goose Up
CREATE TABLE static_configuration (
    network_id varchar(20) NOT NULL,
	address varchar(255) NOT NULL, 
	organization varchar(255) NOT NULL,
    project varchar(255) NOT NULL,
    group_name smallint unsigned,
    smartcontract_name varchar(255) NOT NULL,
    PRIMARY KEY (organization, project, network_id, group_name, smartcontract_name),
    CONSTRAINT smartcontract_id FOREIGN KEY (network_id, address) REFERENCES static_smartcontract(network_id, address) 
);
