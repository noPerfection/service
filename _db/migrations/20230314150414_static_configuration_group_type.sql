-- +goose Up
ALTER TABLE static_configuration MODIFY group_name varchar(127);
