# h0tb0x

h0tb0x is a cloud you make with friends.

## Development Environment

For convenience, there is a script that you can source which will set the `GOPATH` and `PATH`:
```
. dev.env
```

### Mac OS X

Install homebrew:

```
ruby -e "$(curl -fsSL https://raw.github.com/mxcl/homebrew/go/install)"
```

Update your PATH by adding this to your `$HOME/.bashrc` file:

```
export PATH=/usr/local/bin:$PATH
```

Use brew to install prereqs:
```
brew update
brew install apple-gcc42 git-flow go node python bzr
```

### Ubuntu

Use apt-get to install prereqs:
```
apt-get install python-software-properties
add-apt-repository ppa:chris-lea/node.js
apt-get update
apt-get install git-flow nodejs python-pip bzr
```

Download and install prebuilt Linux go binaries (assuming you have a 64-bit machine):
```
apt-get install wget build-essential
wget https://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.1.2.linux-amd64.tar.gz
```

Update your PATH by adding this to your `$HOME/.profile`:
```
export PATH=/usr/local/go/bin:$PATH
```

### Arch

First install the Go compiler:
```
sudo pacman -S community/go
```
Install prereqs via pacman:

```
sudo pacman -S nodejs bzr mercurial python-pip

```

Optionally use AUR to install git-flow. 
```
git-flow: https://aur.archlinux.org/packages/gitflow/
```

## Building h0tb0x

A `Makefile` is included and is used in the traditional way:
```
make
```

If you only want to build the h0tb0x server:
```
make go
```

Or, if you prefer to build just the web app:
```
make web
```

And for the impatient, you can build the h0tb0x server without pulling down dependencies:
```
make quick
```

Likewise, you can quickly build the web app and automatically watch for changes with:
```
make -C web watch
```

## Running h0tb0x

The final binary lives in the `bin` directory. 
Today, it runs in the foreground, eventually it should run as a daemon:

```
bin/h0tb0x
```

## Testing h0tb0x

To run all tests, make sure the h0tb0x server is up and running and do:
```
make test
```

With `dev.env` sourced, you can run individual go module unit tests with:
```
go test h0tb0x/{xxx}     # where {xxx} is the module (i.e. sync)
```

To run the web app unit tests and automatically re-run when files change:
```
make -C web unit-test
```

