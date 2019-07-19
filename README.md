# scale2zero

A project to enable the scaling down to zero capability for cloud foundry applications.

mvn package
java -cp 'db/target/lib/*' liquibase.integration.commandline.Main --url jdbc:postgresql://127.0.0.1/autoscaler --driver=org.postgresql.Driver --changeLogFile=src/autoscaler/apiserver/db/api.db.changelog.yml --password=123 --username=postgres update
java -cp 'db/target/lib/*' liquibase.integration.commandline.Main --url jdbc:postgresql://127.0.0.1/autoscaler --driver=org.postgresql.Driver --changeLogFile=src/autoscaler/routemanager/db/routemanager.db.changelog.yml --password=123 --username=postgres update
