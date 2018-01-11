# go-tools
Go Development tools

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

# TODO:
- add more doc and example
