# Itzo-launcher
A simple helper for launching the right version of itzo.

## Build

    $ make

## Usage

Itzo-launcher should be used via systemd or some other service manager. Example unit file:

    [Unit]
    Description=Itzo launcher
    After=network.target
    StartLimitIntervalSec=0
    
    [Service]
    Type=simple
    Restart=on-failure
    RestartSec=3s
    ExecStart=/usr/bin/itzo-launcher --v=5
    
    [Install]
    WantedBy=multi-user.target

Once an instance is started with itzo-launcher on it, itzo-launcher will check user-data, download the version of itzo requested via the usual itzo user-data files, and start itzo.
