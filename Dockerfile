##
## STEP 1 - BUILD
##

FROM golang:1.23-alpine AS build

WORKDIR /app

RUN apk --no-cache add ca-certificates git coreutils  gammu gammu-smsd gammu-dev gammu-libs

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY ./cmd ./cmd
##COPY ./.git ./.git
COPY ./conf ./conf


##RUN ls .git
RUN  export GITDATE=$(git show --no-patch --no-notes --pretty='%cd' --date=iso)

RUN  go build  -ldflags="-X 'github.com/guigui42/gammuwrapper/build.Version=v`date -d "$(git show --no-patch --no-notes --pretty='%cd' --date=iso)" +%Y.%m.%d`-`git rev-parse --short HEAD`' -X 'github.com/demofl-io/demoflio-cloud/build.Time=`date`' "  -o /gammuwrapper ./cmd/gammuwrapper


EXPOSE 8083

CMD [ "/gammuwrapper" ]