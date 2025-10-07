*The Raft algorithm is very interesting so I will be implementing it.*

Credit goes to this [blog](https://eli.thegreenplace.net/2020/implementing-raft-part-0-introduction/) for guiding me.

## What is a "Raft"?
It's not a boat. The Raft algorithm is a consensus algorithm used to ensure distributed server clusters remain synchronized
between each other even through server failures or network partitions. It does this by electing a leader server, making the rest
of the cluster followers. A client communicating to the cluster does so through the leader. The leader then passes on any
commands the client sends to the followers, ensuring that the followers replicate the leader. 

Should the leader timeout, the followers will then begin elections to re-elect a new leader, beginning the cycle again.
