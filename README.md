go-sensu
========

go-sensu

Sensu Agent written in Go

TODO: 
- check if standalone tasks work properly
- implement local handling for stand alone tasks
- check if rabbit reconnect works properly: do not log.Fatal on reconnect
- implement logging and verbose logging



LongTermTODO and things to think:
- rewrite sensu server in go
- change configs from json to toml


- make templatable configs: 
    example:
      host_list: [host1, host2]
      
      check_host:
        command: /bin/ping {{item}}
        whith:
          [host1, host2, host3]
          
      check_host2:
        command: /bin/ping {{item}}
        whith:
          host_list

- make distributed server. we need to have garantied error handling:
- make consensus checks: 
  checking host availability we often need to have 2 or more witnesses to be sure that there 
  is a problem with our host and not with network from one of the witnesses
  
  check_host
    command: ping example.com
    subscribers: host1, host2
    whitness:
      host1 && host2 # check handles if it fails on one of 
      host1 || host2 # check handles if it fails on both
      host1 XOR host2 # suppose example.com must be available from host1 and not available from host2
                      #check handles if host available from both hosts. 
                      
- make predicate checks:
  
  check_system:
    predicate: check_mem && check_la # check handles if both checks failed, check will schedule on monitoring server
  
  check_mem:
    command: /bin/check_mem.sh
    subscribers: host1
    
  check_la:
    command: /bin/check_la.sh
    subscribers: host1
