%%%-------------------------------------------------------------------
%% @doc farm_parallel_pattern public API
%% @end
%%%-------------------------------------------------------------------

-module(farm_parallel_pattern_app).

-behaviour(application).

-export([start/2, stop/1]).

start(_StartType, _StartArgs) ->
    farm_parallel_pattern_sup:start_link().

stop(_State) ->
    ok.

%% internal functions
