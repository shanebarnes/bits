#go-server
Run a pool of HTTP servers


##Examples:

###IPv4-only

`
./go-server -addr http://127.0.0.1:80
`

###IPv6-only

`
./go-server -addr http://[::1]:80
`

###Dual-Stack IP on any address

`
./go-server -addr http://:80
`

###Address Pool
`
./go-server -addr http://:80 -addr https://:443 -cert <certificate-file-path> -key <key-file-path>
`
