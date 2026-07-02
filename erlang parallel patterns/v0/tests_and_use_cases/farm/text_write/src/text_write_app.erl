%%%-------------------------------------------------------------------
%% @doc text_write public API
%% @end
%%%-------------------------------------------------------------------

-module(text_write_app).

-behaviour(application).

-export([start/2, stop/1]).

start(_StartType, _StartArgs) ->
    text_write_sup:start_link().

stop(_State) ->
    ok.

%% internal functions
