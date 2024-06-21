resource "local_file" "ssh_key" {
  content         = tls_private_key.ssh.private_key_openssh
  filename        = "${path.module}/secrets/id_rsa.pem"
  file_permission = "0400"
}