package daemon

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"text/template"
)

type LinuxRecord struct {
	name        string
	description string
}

func newDaemon(name, description string) (*LinuxRecord, error) {

	return &LinuxRecord{name, description}, nil
}

// Standard service path for system V daemons
func (linux *LinuxRecord) servicePath() string {
	return "/etc/init.d/" + linux.name
}

// Check service is running
func (linux *LinuxRecord) checkStatus() (string, bool) {
	output, err := exec.Command("service", linux.name, "status").Output()
	if err == nil {
		if matched, err := regexp.MatchString(linux.name, string(output)); err == nil && matched {
			reg := regexp.MustCompile("pid  ([0-9]+)")
			data := reg.FindStringSubmatch(string(output))
			if len(data) > 1 {
				return "Service (pid  " + data[1] + ") is running...", true
			} else {
				return "Service is running...", true
			}
		}
	}

	return "Service is stoped", false
}

func (linux *LinuxRecord) Install() (string, error) {
	installAction := "Install " + linux.description + ":"

	if checkPrivileges() == false {
		return installAction + failed, errors.New(rootPrivileges)
	}

	srvPath := linux.servicePath()

	if _, err := os.Stat(srvPath); err == nil {
		return installAction + failed, errors.New(linux.description + " already installed")
	}

	file, err := os.Create(srvPath)
	if err != nil {
		return installAction + failed, err
	}
	defer file.Close()

	execPatch, err := executablePath()
	if err != nil {
		return installAction + failed, err
	}

	templ, err := template.New("daemonConfig").Parse(daemonConfig)
	if err != nil {
		return installAction + failed, err
	}

	if err := templ.Execute(
		file,
		&struct {
			Name, Description, Path string
		}{linux.name, linux.description, execPatch},
	); err != nil {
		return installAction + failed, err
	}

	if err := os.Chmod(srvPath, 0755); err != nil {
		return installAction + failed, err
	}

	for _, i := range [...]string{"2", "3", "4", "5"} {
		if err := os.Symlink(srvPath, "/etc/rc"+i+".d/S87"+linux.name); err != nil {
			continue
		}
	}
	for _, i := range [...]string{"0", "1", "6"} {
		if err := os.Symlink(srvPath, "/etc/rc"+i+".d/K17"+linux.name); err != nil {
			continue
		}
	}

	return installAction + success, nil
}

func (linux *LinuxRecord) Remove() (string, error) {
	removeAction := "Removing " + linux.description + ":"

	if checkPrivileges() == false {
		return removeAction + failed, errors.New(rootPrivileges)
	}

	if err := os.Remove(linux.servicePath()); err != nil {
		return removeAction + failed, err
	}

	for _, i := range [...]string{"2", "3", "4", "5"} {
		if err := os.Remove("/etc/rc" + i + ".d/S87" + linux.name); err != nil {
			continue
		}
	}
	for _, i := range [...]string{"0", "1", "6"} {
		if err := os.Remove("/etc/rc" + i + ".d/K17" + linux.name); err != nil {
			continue
		}
	}

	return removeAction + success, nil
}

func (linux *LinuxRecord) Start() (string, error) {
	startAction := "Starting " + linux.description + ":"

	if checkPrivileges() == false {
		return startAction + failed, errors.New(rootPrivileges)
	}

	if _, status := linux.checkStatus(); status == true {
		return startAction + failed, errors.New("service already running")
	}

	if err := exec.Command("service", linux.name, "start").Run(); err != nil {
		return startAction + failed, err
	}

	return startAction + success, nil
}

func (linux *LinuxRecord) Stop() (string, error) {
	stopAction := "Stopping " + linux.description + ":"

	if checkPrivileges() == false {
		return stopAction + failed, errors.New(rootPrivileges)
	}

	if _, status := linux.checkStatus(); status == false {
		return stopAction + failed, errors.New("service already stopped")
	}

	if err := exec.Command("service", linux.name, "stop").Run(); err != nil {
		return stopAction + failed, err
	}

	return stopAction + success, nil
}

func (linux *LinuxRecord) Status() (string, error) {

	if checkPrivileges() == false {
		return "", errors.New(rootPrivileges)
	}
	statusAction, _ := linux.checkStatus()

	return statusAction, nil
}

var daemonConfig = `#! /bin/sh
#
#       /etc/rc.d/init.d/{{.Name}}
#
#       Starts {{.Name}} as a daemon
#
# chkconfig: 2345 87 17
# description: Starts and stops a single {{.Name}} instance on this system

### BEGIN INIT INFO
# Provides: {{.Name}} 
# Required-Start: $network $named
# Required-Stop: $network $named
# Default-Start: 2 3 4 5
# Default-Stop: 0 1 6
# Short-Description: This service manages the {{.Description}}.
# Description: {{.Description}} - A quick and easy way to setup your own WHOIS server.
### END INIT INFO

#
# Source function library.
#
if [ -f /etc/rc.d/init.d/functions ]; then
    . /etc/rc.d/init.d/functions
fi

exec="{{.Path}}"
servname="{{.Description}}"

proc=$(basename $0)
pidfile="/var/run/$proc.pid"
lockfile="/var/lock/subsys/$proc"
stdoutlog="/var/log/$proc.log"
stderrlog="/var/log/$proc.err"

[ -e /etc/sysconfig/$proc ] && . /etc/sysconfig/$proc

start() {
    [ -x $exec ] || exit 5

    if ! [ -f $pidfile ]; then
        printf "Starting $servname:\t"
        echo "$(date)" >> $stdoutlog
        $exec >> $stderrlog 2>> $stdoutlog &
        echo $! > $pidfile
        touch $lockfile
        success
        echo
    else
        failure
        echo
        printf "$pidfile still exists...\n"
        exit 7
    fi
}

stop() {
    echo -n $"Stopping $servname: "
    killproc -p $pidfile $proc
    retval=$?
    echo
    [ $retval -eq 0 ] && rm -f $lockfile
    return $retval
}

restart() {
    stop
    start
}

rh_status() {
    status -p $pidfile $proc
}

rh_status_q() {
    rh_status >/dev/null 2>&1
}

case "$1" in
    start)
        rh_status_q && exit 0
        $1
        ;;
    stop)
        rh_status_q || exit 0
        $1
        ;;
    restart)
        $1
        ;;
    status)
        rh_status
        ;;
    *)
        echo $"Usage: $0 {start|stop|status|restart}"
        exit 2
esac

exit $?
`
