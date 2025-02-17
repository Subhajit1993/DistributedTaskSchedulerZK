FROM zookeeper:3.8.3

# Copy custom configuration if needed
COPY zoo.cfg /conf/zoo.cfg

# Default ports:
# 2181 - client connections
# 2888 - follower connections
# 3888 - election
EXPOSE 2181 2888 3888

# Environment variables you can override
ENV ZOO_MY_ID=1
ENV ZOO_SERVERS=server.1=localhost:2888:3888;2181

# Data directory
VOLUME ["/data", "/datalog", "/logs"]