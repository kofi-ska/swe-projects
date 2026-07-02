-module(text_write_parallel).
-export([write_to_file_parallel/3, create_or_overwrite_file/1, run/1]).

% Function to create or overwrite the file
create_or_overwrite_file(Filename) ->
    % Attempt to open the file in write mode (creates or overwrites)
    case file:open(Filename, [write]) of
        {ok, File} ->
            % Successfully opened the file
            file:close(File),
            ok;
        {error, Reason} ->
            % Return an error if unable to open the file
            {error, Reason}
    end.

% Function to write text to a file in parallel with the number of available cores
write_to_file_parallel(Filename, Text, Times) ->
    % Create or overwrite the file
    case create_or_overwrite_file(Filename) of
        ok ->
            % Get the number of logical processors (cores)
            NumWorkers = erlang:system_info(logical_processors),
            % Start the workers
            Pids = start_workers(NumWorkers, Filename, Text, Times),
            % Wait for all workers to finish
            wait_for_workers(Pids),
            ok;
        {error, Reason} ->
            {error, Reason}
    end.

% Start worker processes
start_workers(NumWorkers, Filename, Text, Times) ->
    % Calculate how many writes each worker should do
    WritesPerWorker = Times div NumWorkers,
    Remainder = Times rem NumWorkers,
    
    % Create workers
    lists:map(fun(Index) ->
        spawn(fun() ->
            WorkerId = Index,
            % Each worker does its share of writes
            WriteCount = WritesPerWorker + (if WorkerId =< Remainder -> 1; true -> 0 end),
            write_worker(Filename, Text, WriteCount)
        end)
    end, lists:seq(1, NumWorkers)).

% Worker function to write text to a file
write_worker(Filename, Text, Times) ->
    case file:open(Filename, [write, append]) of
        {ok, File} ->
            write_text(File, Text, Times),
            file:close(File);
        {error, Reason} ->
            io:format("Worker failed to open file: ~p~n", [Reason])
    end.

% Helper function to write text N times
write_text(_, _, 0) ->
    ok;
write_text(File, Text, N) when N > 0 ->
    io:format(File, "~s~n", [Text]),
    write_text(File, Text, N - 1).

% Wait for all worker processes to finish using process monitoring
wait_for_workers(Pids) ->
    %% Monitor all PIDs and wait for them to exit
    lists:foreach(fun(Pid) ->
        monitor(process, Pid),
        receive
            {'DOWN', _Ref, process, Pid, _Reason} -> ok
        end
    end, Pids).

% Run function to take filename, text, and number of times
run(Times) ->
    Filename = "output_parallel.txt",
    Text = "Hello, World!",
    
    % Write the text to the file in parallel
    case write_to_file_parallel(Filename, Text, Times) of
        ok ->
            io:format("File written successfully in parallel.~n");
        {error, Reason} ->
            io:format("Failed to write to file: ~p~n", [Reason])
    end.
