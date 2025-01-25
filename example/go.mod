module github.com/mateusf777/pubsub/example

go 1.23.4

require github.com/mateusf777/pubsub/client v0.1.4

require github.com/mateusf777/pubsub/core v0.1.3 // indirect

replace github.com/mateusf777/pubsub/client v0.1.4 => ../client
replace github.com/mateusf777/pubsub/core v0.1.3 => ../core