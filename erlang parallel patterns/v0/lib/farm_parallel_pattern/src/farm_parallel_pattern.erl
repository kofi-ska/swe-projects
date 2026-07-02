-module(farm_parallel_pattern).
-export([farm_work/4, create_or_overwrite_file/1]).

% Function to create or overwrite the file
create_or_overwrite_file(Filename) ->
    case file:open(Filename, [write]) of
        {ok, File} ->
            file:close(File),
            ok;
        {error, Reason} ->
            {error, Reason}
    end.

% Function to split the workload and distribute it across workers
split_work(Workload, NumWorkers, WorkerFun) ->
    UnitsPerWorker = Workload div NumWorkers,
    Remainder = Workload rem NumWorkers,

    Pids = lists:map(fun(Index) ->
        spawn(fun() ->
            WorkerUnits = UnitsPerWorker + (if Index =< Remainder -> 1; true -> 0 end),
            WorkerFun(Index, WorkerUnits),
            % Terminate after completing the task
            exit(normal)
        end)
    end, lists:seq(1, NumWorkers)),

    Pids.

% Wait for all worker processes to finish using process monitoring
wait_for_workers(Pids) ->
    %% Monitor all PIDs and wait for them to exit
    lists:foreach(fun(Pid) ->
        monitor(process, Pid),
        receive
            {'DOWN', _Ref, process, Pid, _Reason} -> ok
        end
    end, Pids).

% General function to perform parallel work
farm_work(Workload, NumWorkers, WorkerFun, Filename) ->
    case create_or_overwrite_file(Filename) of
        ok ->
            Pids = split_work(Workload, NumWorkers, fun(_WorkerId, Units) ->
                WorkerFun(Filename, Units)
            end),
            wait_for_workers(Pids),
            ok;
        {error, Reason} ->
            {error, Reason}
    end.

