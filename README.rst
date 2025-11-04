OhDear Health Check
-------------------

This tool acts as the "Health Report URL" endpoint for the "Application
health" monitor for `OhDear <https://ohdear.app/>`_.

It will listen on port ``8991`` by default and can be configure with the
``ohdear-health.yaml`` configuration file that is being read from the current
working directory.

Configuration
~~~~~~~~~~~~~

In this file, you can configure the Warning and Error thresholds for the
``LoadAverage``, ``MemoryUsagePercent`` and ``DiskUsagePercent`` checks. The
Load Average is the 5 minute load average, and the configuration for
``MemoryUsagePercent`` and ``DiskUsagePercent`` is in percent (range: 0-100).

You can configure the last two of these like::

	MemoryUsagePercent:
	  Warning: 80
	  Error: 90

Beyond these three, you can also optionally configure ``TCPService`` to check
for. The tool will make a TCP/IP connection to ``Port`` for each service, and
then instantly disconnect. If it can't connect, it will raise an ``Error``.

To check for MongoDB running, you would add::

	TCPServices:
	- Description: "MongoDB"
	  Port: "localhost:27017"

The configuration file is re-read upon every incoming health-check request, so
you don't have to restart the daemon manually.

Running The Service
~~~~~~~~~~~~~~~~~~~

To have the binary running as a daemon, save the following file in
``/etc/systemd/system/ohdear-health-check.service``, making sure to update the
``ExecStart`` value to the location of the binary, and the
``WorkingDirectory`` where your ``ohdear-health.yaml`` configuration file
resides::

	[Unit]
	Description=OhDear Health Check
	After=network.target

	[Service]
	Type=exec
	ExecStart=/local/systems/bin/ohdear-system-health
	WorkingDirectory=/local/systems/bin
	Restart=always

Then reload systemd's services with ``systemctl daemon-reload``, after which
you can start the service with ``systemctl start
ohdear-health-check.service``.
