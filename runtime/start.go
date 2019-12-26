package runtime

func (b *boxRuntime) Start(pid int) error {
	// TODO: currently there is no way to know if the child process has died for sure.
	//  runC uses the start time of the process to make sure it is the right process
	//  but for now I'm not storing the box state yet
	// first check if the waiting child is still alive
	//stat, err := system.Stat(pid)
	//if err != nil {
	//	return fmt.Errorf("container is stopped")
	//}
	//if stat.StartTime != c.initProcessStartTime ||
	//	stat.State == system.Zombie ||
	//	stat.State == system.Dead {
	//	return fmt.Errorf("container is stopped")
	//}

	// TODO: must be thread safe

	b.childProcess = process{
		pid:    pid,
		config: b.childProcess.config,
	}

	return b.exec()
}
