-- +goose Up
CREATE TABLE configuration (
    network_id varchar(20) NOT NULL,
	address varchar(84) NOT NULL, 
	organization varchar(127) NOT NULL,
    project varchar(127) NOT NULL,
    group_name smallint unsigned,
    smartcontract_name varchar(127) NOT NULL,
    PRIMARY KEY (organization, project, network_id, group_name, smartcontract_name),
    CONSTRAINT smartcontract_id FOREIGN KEY (network_id, address) REFERENCES smartcontract(network_id, address) 
);
