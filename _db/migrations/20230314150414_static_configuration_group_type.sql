-- +goose Up
ALTER TABLE configuration MODIFY group_name varchar(127);
