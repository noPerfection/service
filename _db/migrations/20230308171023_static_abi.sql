-- +goose Up
-- +goose StatementBegin
CREATE TABLE abi (
    abi_id varchar(20) NOT NULL,
    body json,
    UNIQUE KEY(abi_id)
);
-- +goose StatementEnd
