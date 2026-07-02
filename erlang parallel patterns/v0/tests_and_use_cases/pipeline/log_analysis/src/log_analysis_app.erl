%%%-------------------------------------------------------------------
%% @doc log_analysis public API
%% @end
%%%-------------------------------------------------------------------

-module(log_analysis_app).

-behaviour(application).

-export([start/2, stop/1]).

start(_StartType, _StartArgs) ->
    log_analysis_sup:start_link().

stop(_State) ->
    ok.

%% internal functions
