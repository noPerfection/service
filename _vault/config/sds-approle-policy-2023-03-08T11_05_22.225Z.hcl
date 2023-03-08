path "auth/sds-approle/login" {
  capabilities = ["create", "read"]
}

path "sds-mysql/creds/sds-mysql-role" {
  capabilities = ["read", "create", "list", "update", "delete"]
}

path "sds-auth-kv/curves/" {
  capabilities = ["read", "create", "list", "update", "delete"]
}