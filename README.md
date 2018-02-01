# go-tools
Go Development tools

## how to install

    go get -u github.com/payfazz/go-tools/cmd/...

## cmd/gp
    Move project to GOPATH, based on import path comment
    see: https://golang.org/cmd/go/#hdr-Import_path_checking

    USAGE:
    gp init [-f] [TargetPath]    move the project to GOPATH and replace it with symlink
                                 if TargetPath is not specified, it will
                                 run "go list -f '{{.ImportComment}}'"
                                 "-f" will remove what already inside GOPATH
    gp deinit                    move out project from GOPATH
    gp fix <BrokenPath>          rewrite go source code recursively, replace BrokenPath
                                 with project ImportPath
    gp list-ext                  list external dependencies
    gp status                    show status

let say you clone this repo ouside GOPATH, running `gp init` will move the directory to `$GOPATH/github.com/payfazz/go-tools` and create symlink point to that directory

running `gp deinit` will do the reverse

# TODO:
- add more doc and example
