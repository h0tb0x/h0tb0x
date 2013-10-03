# `h0tb0x`

`h0tb0x` is a socially distributed file system.

## Development Environment

### Mac OS X

Use brew to install prereqs:
```
brew update
brew install apple-gcc42 git-flow go node
```

### Ubuntu

Use apt-get to install prereqs:
```
apt-get install python-software-properties
add-apt-repository ppa:chris-lea/node.js
apt-get update
apt-get install git-flow nodejs
```

Download and build go 1.1.2 from source (assuming you have a 64-bit machine):
```
apt-get install wget build-essential
wget https://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.1.2.linux-amd64.tar.gz
```

Add this to your `$HOME/.profile`:
```
export PATH=$PATH:/usr/local/go/bin
```

## Building `h0tb0x`

A `Makefile` is included and is used in the traditional way:
```
make
```

To run tests after you've built it, try:
```
make test
```

If you only want to build the web app:
```
make web
```

And if you want to be sure that you're using the latest dependencies:
```
make clean
make
```

## Running `h0tb0x`

The final binary lives in the `bin` directory. 
Today, it runs in the foreground, eventually it should run as a daemon:

```
bin/h0tb0x
```
