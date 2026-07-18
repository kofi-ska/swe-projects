-module(pipeline_parallel_pattern).
-export([run_pipeline/2]).

%% Public API function
run_pipeline(Stages, InitialInput) ->
    %% Start the pipeline with the initial input
    run_pipeline_stage(Stages, InitialInput, self()).

%% Base case: no more stages
run_pipeline_stage([], Input, ParentPid) ->
    %% No more stages, send final result to the parent process
    ParentPid ! {pipeline_done, Input},
    receive
        {pipeline_done, Result} -> {ok, Result}
    end;

%% Recursive case: apply the current stage and proceed to the next
run_pipeline_stage([Stage | Rest], Input, ParentPid) ->
    %% Start the stage process and monitor it
    Pid = spawn(fun() ->
        Result = Stage(Input),
        ParentPid ! {stage_done, Result}
    end),
    Ref = erlang:monitor(process, Pid),

    %% Wait for this stage to complete, either normally or with an error
    receive
        {'DOWN', Ref, process, _Pid, Reason} ->
            %% Handle process termination
            ParentPid ! {stage_failed, Reason};
        {stage_done, Result} ->
            %% Recursively process the rest of the stages
            erlang:demonitor(Ref, [flush]),
            run_pipeline_stage(Rest, Result, ParentPid)
    end.