FROM debian:buster-slim
RUN useradd -u 1000 ssdb

FROM golang:latest
COPY --from=0 /etc/passwd /etc/passwd
COPY ssdb_exporter /opt/ssdb_exporter
RUN cd /opt/ssdb_exporter && go build -o ssdb_exporter && ln -s /opt/ssdb_exporter/ssdb_exporter /usr/bin/ssdb_exporter
RUN chown -R ssdb /opt/ssdb_exporter
EXPOSE 9142
#ENTRYPOINT [ "/usr/bin/ssdb_exporter" ]
