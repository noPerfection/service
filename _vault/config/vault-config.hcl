listener "tcp" {
    address = "0.0.0.0:8200"
    tls_disable = "true"
}

storage "raft" {
    path = "/vault/file"
    node_id="raft_node1"
}

plugin_directory="/vault/plugins"
cluster_addr = "http://127.0.0.1:8201"
disable_mlock = "true"
ui = "true"