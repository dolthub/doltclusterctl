package main

// A kludge to take a dependency on this binary for our push script.

import (
	_ "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
)

func main() {
}
